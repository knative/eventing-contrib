/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	ghclient "github.com/google/go-github/github"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	ghreconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
	"knative.dev/eventing-contrib/github/pkg/common"
	"knative.dev/eventing-contrib/github/pkg/reconciler/source/resources"
)

const (
	saName        = "github-adapter"
	adapterName   = "githubsource-adapter"
	raImageEnvVar = "GH_RA_IMAGE"

	GitHubSourceServiceCreated = "GitHubSourceServiceCreated"
	GitHubSourceServiceUpdated = "GitHubSourceServiceUpdated"
)

// Reconciler reconciles a GitHubSource object
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	servingClientSet servingclientset.Interface
	servingLister    servinglisters.ServiceLister

	receiveAdapterImage string

	// tracking mt adapter ksvc changes
	tracker tracker.Interface

	sinkResolver *resolver.URIResolver

	webhookClient webhookClient
}

var _ ghreconciler.Interface = (*Reconciler)(nil)
var _ ghreconciler.Finalizer = (*Reconciler)(nil)

type webhookArgs struct {
	source                *sourcesv1alpha1.GitHubSource
	url                   *apis.URL
	accessToken           string
	secretToken           string
	alternateGitHubAPIURL string
	hookID                string
}

func (r *Reconciler) ReconcileKind(ctx context.Context, source *sourcesv1alpha1.GitHubSource) pkgreconciler.Event {
	source.Status.InitializeConditions()

	accessToken, err := common.SecretFrom(r.kubeClientSet, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
		return err
	}
	secretToken, err := common.SecretFrom(r.kubeClientSet, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("SecretTokenNotFound", "%s", err)
		return err
	}
	source.Status.MarkSecrets()

	dest := source.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if dest.Ref.Namespace == "" {
			dest.Ref.Namespace = source.GetNamespace()
		}
	}

	uri, err := r.sinkResolver.URIFromDestinationV1(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.reconcileReceiveAdapter(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the mt receive adapter", zap.Error(err))
		return err
	}

	// Tell tracker to reconcile this GitHubSource whenever the Kservice changes
	err = r.tracker.TrackReference(tracker.Reference{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
		Namespace:  ksvc.Namespace,
		Name:       ksvc.Name,
	}, source)

	if err != nil {
		logging.FromContext(ctx).Error("Unable to track the deployment", zap.Error(err))
		return err
	}

	withPath := *ksvc.Status.URL
	withPath.Path = fmt.Sprintf("/%s/%s", source.Namespace, source.Name)

	if ksvc.Status.IsReady() && ksvc.Status.URL != nil {
		args := &webhookArgs{
			source:                source,
			url:                   &withPath,
			accessToken:           accessToken,
			secretToken:           secretToken,
			alternateGitHubAPIURL: source.Spec.GitHubAPIURL,
		}

		// source.Status.MarkServiceDeployed(ra)
		// TODO: Mark Deployed for the ksvc
		if source.Status.WebhookIDKey == "" {
			hookID, err := r.createWebhook(ctx, args)
			if err != nil {
				source.Status.MarkWebhookNotConfigured("CreationFailed", err.Error())
				return err
			}
			source.Status.WebhookIDKey = hookID
		} else {
			err := r.reconcileWebhook(ctx, args, source.Status.WebhookIDKey)
			if err != nil {
				source.Status.MarkWebhookNotConfigured("ReconciliationFailed", err.Error())
				return err
			}
		}
		source.Status.MarkWebhookConfigured()
	}
	source.Status.CloudEventAttributes = r.createCloudEventAttributes(source)
	source.Status.ObservedGeneration = source.Generation
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *sourcesv1alpha1.GitHubSource) pkgreconciler.Event {
	// If a webhook was created, try to delete it
	if source.Status.WebhookIDKey != "" {
		// Get access token
		accessToken, err := common.SecretFrom(r.kubeClientSet, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if apierrors.IsNotFound(err) {
			source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
			controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning,
				"WebhookDeletionSkipped", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			// return EventTypeNormal to avoid finalize loop
			return pkgreconciler.NewEvent(corev1.EventTypeNormal, "WebhookDeletionSkipped", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
		} else if err != nil {
			controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning,
				"WebhookDeletionFailed", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return fmt.Errorf("error getting secret: %v", err)
		}

		args := &webhookArgs{
			source:                source,
			accessToken:           accessToken,
			alternateGitHubAPIURL: source.Spec.GitHubAPIURL,
			hookID:                source.Status.WebhookIDKey,
		}
		// Delete the webhook using the access token and stored webhook ID
		err = r.deleteWebhook(ctx, args)
		var gherr *ghclient.ErrorResponse
		if errors.As(err, &gherr) {
			if gherr.Response.StatusCode == http.StatusNotFound {
				controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning, "WebhookDeletionSkipped", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
				// return EventTypeNormal to avoid finalize loop
				return pkgreconciler.NewEvent(corev1.EventTypeNormal, "WebhookDeletionSkipped", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			}
		} else {
			controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning,
				"WebhookDeletionFailed", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return fmt.Errorf("error deleting webhook: %v", err)
		}
		// Webhook deleted, clear ID
		source.Status.WebhookIDKey = ""
	}
	return nil
}

func (r *Reconciler) reconcileReceiveAdapter(ctx context.Context, source *v1alpha1.GitHubSource) (*servingv1.Service, error) {
	expected := resources.MakeService(&resources.ServiceArgs{
		ServiceAccountName:  saName,
		ReceiveAdapterImage: r.receiveAdapterImage,
		AdapterName:         adapterName,
	})

	s, err := r.servingLister.Services(system.Namespace()).Get(adapterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			s, err := r.servingClientSet.ServingV1().Services(system.Namespace()).Create(expected)
			if err != nil {
				controller.GetEventRecorder(ctx).Eventf(source, corev1.EventTypeWarning, GitHubSourceServiceCreated, "Cluster-scoped Knative service not created (%v)", err)
				return nil, err
			}
			controller.GetEventRecorder(ctx).Event(source, corev1.EventTypeNormal, GitHubSourceServiceCreated, "Cluster-scoped Knative service created")
			return s, nil
		}
		return nil, fmt.Errorf("error getting mt adapter Knative service %v", err)
	} else if podSpecChanged(s.Spec.Template.Spec.PodSpec, expected.Spec.Template.Spec.PodSpec) {
		s.Spec.Template.Spec = expected.Spec.Template.Spec
		if s, err = r.servingClientSet.ServingV1().Services(system.Namespace()).Update(s); err != nil {
			return s, err
		}
		controller.GetEventRecorder(ctx).Event(source, corev1.EventTypeNormal, GitHubSourceServiceUpdated, "Cluster-scoped Knative service updated")
		return s, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing cluster-scoped Knative service", zap.Any("ksvc", s))
	}
	return s, nil
}

func (r *Reconciler) createWebhook(ctx context.Context, args *webhookArgs) (string, error) {
	logger := logging.FromContext(ctx)

	logger.Info("creating GitHub webhook")

	owner, repo, err := parseOwnerRepoFrom(args.source.Spec.OwnerAndRepository)
	if err != nil {
		return "", err
	}

	url := args.url
	if args.url != nil && args.source.Spec.Secure != nil {
		// Make a copy
		u := *args.url
		url = &u

		if *args.source.Spec.Secure {
			url.Scheme = "https"
		} else {
			url.Scheme = "http"
		}
	}

	hookOptions := &webhookOptions{
		accessToken: args.accessToken,
		secretToken: args.secretToken,
		url:         url,
		owner:       owner,
		repo:        repo,
		events:      args.source.Spec.EventTypes,
	}

	hookID, err := r.webhookClient.Create(ctx, hookOptions, args.alternateGitHubAPIURL)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook: %v", err)
	}
	return hookID, nil
}

func (r *Reconciler) reconcileWebhook(ctx context.Context, args *webhookArgs, hookID string) error {
	logger := logging.FromContext(ctx)

	logger.Info("reconciling GitHub webhook")

	owner, repo, err := parseOwnerRepoFrom(args.source.Spec.OwnerAndRepository)
	if err != nil {
		return err
	}

	url := args.url
	if args.url != nil && args.source.Spec.Secure != nil {
		// Make a copy
		u := *args.url
		url = &u

		if *args.source.Spec.Secure {
			url.Scheme = "https"
		} else {
			url.Scheme = "https"
		}
	}

	hookOptions := &webhookOptions{
		accessToken: args.accessToken,
		secretToken: args.secretToken,
		url:         url,
		owner:       owner,
		repo:        repo,
		events:      args.source.Spec.EventTypes,
	}

	if err := r.webhookClient.Reconcile(ctx, hookOptions, hookID, args.alternateGitHubAPIURL); err != nil {
		return fmt.Errorf("failed to reconcile webhook: %v", err)
	}
	return nil
}

func (r *Reconciler) deleteWebhook(ctx context.Context, args *webhookArgs) error {
	logger := logging.FromContext(ctx)

	logger.Info("deleting GitHub webhook")

	owner, repo, err := parseOwnerRepoFrom(args.source.Spec.OwnerAndRepository)
	if err != nil {
		return err
	}

	hookOptions := &webhookOptions{
		accessToken: args.accessToken,
		owner:       owner,
		repo:        repo,
		events:      args.source.Spec.EventTypes,
	}
	err = r.webhookClient.Delete(ctx, hookOptions, args.hookID, args.alternateGitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}

func parseOwnerRepoFrom(ownerAndRepository string) (string, string, error) {
	components := strings.Split(ownerAndRepository, "/")
	if len(components) > 2 {
		return "", "", fmt.Errorf("ownerAndRepository is malformatted, expected 'owner/repository' but found %q", ownerAndRepository)
	}
	owner := components[0]
	if len(owner) == 0 && len(components) > 1 {
		return "", "", fmt.Errorf("owner is empty, expected 'owner/repository' but found %q", ownerAndRepository)
	}
	repo := ""
	if len(components) > 1 {
		repo = components[1]
	}

	return owner, repo, nil
}

func (r *Reconciler) createCloudEventAttributes(src *sourcesv1alpha1.GitHubSource) []duckv1.CloudEventAttributes {
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(src.Spec.EventTypes))
	for _, ghType := range src.Spec.EventTypes {
		ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
			Type:   sourcesv1alpha1.GitHubEventType(ghType),
			Source: sourcesv1alpha1.GitHubEventSource(src.Spec.OwnerAndRepository),
		})
	}
	return ceAttributes
}

func podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	if !equality.Semantic.DeepDerivative(newPodSpec, oldPodSpec) {
		return true
	}
	if len(oldPodSpec.Containers) != len(newPodSpec.Containers) {
		return true
	}
	for i := range newPodSpec.Containers {
		if !equality.Semantic.DeepEqual(newPodSpec.Containers[i].Env, oldPodSpec.Containers[i].Env) {
			return true
		}
	}
	return false
}
