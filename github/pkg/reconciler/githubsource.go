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

package github

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	//k8s.io imports
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	//knative.dev/serving imports
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	//knative.dev/eventing-contrib imports
	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	ghreconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
	"knative.dev/eventing-contrib/github/pkg/reconciler/resources"

	//knative.dev/eventing imports
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"

	//knative.dev/pkg imports
	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "github-source-controller"
	raImageEnvVar       = "GH_RA_IMAGE"
)

// Reconciler reconciles a GitHubSource object
type Reconciler struct {
	*reconciler.Base

	servingClientSet servingclientset.Interface
	servingLister    servinglisters.ServiceLister
	eventTypeLister  eventinglisters.EventTypeLister

	receiveAdapterImage string

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

	accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
		return err
	}
	secretToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
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
			//TODO how does this work with deprecated fields
			dest.Ref.Namespace = source.GetNamespace()
		}
	} else if dest.DeprecatedName != "" && dest.DeprecatedNamespace == "" {
		// If Ref is nil and the deprecated ref is present, we need to check for
		// DeprecatedNamespace. This can be removed when DeprecatedNamespace is
		// removed.
		dest.DeprecatedNamespace = source.GetNamespace()
	}
	uri, err := r.sinkResolver.URIFromDestination(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	if source.Spec.Sink.DeprecatedAPIVersion != "" &&
		source.Spec.Sink.DeprecatedKind != "" &&
		source.Spec.Sink.DeprecatedName != "" {
		source.Status.MarkSinkWarnRefDeprecated(uri)
	} else {
		source.Status.MarkSink(uri)
	}

	ksvc, err := r.getOwnedService(ctx, source)
	if apierrors.IsNotFound(err) {
		ksvc = resources.MakeService(&resources.ServiceArgs{
			Source:              source,
			ReceiveAdapterImage: r.receiveAdapterImage,
		})
		ksvc, err = r.servingClientSet.ServingV1().Services(source.Namespace).Create(ksvc)
		if err != nil {
			return err
		}
		r.Recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
		// TODO: Mark Deploying for the ksvc
		// Wait for the Service to get a status
	} else if err != nil {
		// Error was something other than NotFound
		return err
	} else if !metav1.IsControlledBy(ksvc, source) {
		return fmt.Errorf("Service %q is not owned by GitHubSource %q", ksvc.Name, source.Name)
	}

	if ksvc.Status.IsReady() && ksvc.Status.URL != nil {
		args := &webhookArgs{
			source:                source,
			url:                   ksvc.Status.URL,
			accessToken:           accessToken,
			secretToken:           secretToken,
			alternateGitHubAPIURL: source.Spec.GitHubAPIURL,
		}

		// source.Status.MarkServiceDeployed(ra)
		// TODO: Mark Deployed for the ksvc
		// TODO: Mark some condition for the webhook status?
		if source.Status.WebhookIDKey == "" {
			hookID, err := r.createWebhook(ctx, args)
			if err != nil {
				return err
			}
			source.Status.WebhookIDKey = hookID
		} else {
			err := r.reconcileWebhook(ctx, args, source.Status.WebhookIDKey)
			if err != nil {
				return err
			}
		}
	}

	err = r.reconcileEventTypes(ctx, source)
	if err != nil {
		// should we mark no event types here in stat
		return err
	}
	source.Status.MarkEventTypes()

	source.Status.ObservedGeneration = source.Generation
	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *sourcesv1alpha1.GitHubSource) pkgreconciler.Event {
	// If a webhook was created, try to delete it
	if source.Status.WebhookIDKey != "" {
		// Get access token
		accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
			r.Recorder.Eventf(source, corev1.EventTypeWarning,
				"FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return err
		}

		args := &webhookArgs{
			source:                source,
			accessToken:           accessToken,
			alternateGitHubAPIURL: source.Spec.GitHubAPIURL,
			hookID:                source.Status.WebhookIDKey,
		}
		// Delete the webhook using the access token and stored webhook ID
		err = r.deleteWebhook(ctx, args)
		if err != nil {
			r.Recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return err
		}
		// Webhook deleted, clear ID
		source.Status.WebhookIDKey = ""
	}
	return nil
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
		return fmt.Errorf("failed to delete webhook: %v", err)
	}
	return nil
}

func (r *Reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	secret, err := r.KubeClientSet.CoreV1().Secrets(namespace).Get(secretKeySelector.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
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

func (r *Reconciler) getOwnedService(ctx context.Context, source *sourcesv1alpha1.GitHubSource) (*v1.Service, error) {
	serviceList, err := r.servingLister.Services(source.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, ksvc := range serviceList {
		if metav1.IsControlledBy(ksvc, source) {
			//TODO if there are >1 controlled, delete all but first?
			return ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(v1.Resource("services"), "")
}

func (r *Reconciler) reconcileEventTypes(ctx context.Context, source *sourcesv1alpha1.GitHubSource) error {
	current, err := r.getEventTypes(ctx, source)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get existing event types", zap.Error(err))
		return err
	}

	expected, err := r.newEventTypes(source)
	if err != nil {
		return err
	}

	toCreate, toDelete := r.computeDiff(current, expected) // will need to add this

	for _, eventType := range toDelete {
		if err = r.EventingClientSet.EventingV1alpha1().EventTypes(source.Namespace).Delete(eventType.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("Error deleting eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	for _, eventType := range toCreate {
		if _, err = r.EventingClientSet.EventingV1alpha1().EventTypes(source.Namespace).Create(&eventType); err != nil {
			logging.FromContext(ctx).Error("Error creating eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	return err
}

func (r *Reconciler) newEventTypes(source *sourcesv1alpha1.GitHubSource) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0, len(source.Spec.EventTypes))

	if ref := source.Spec.Sink.GetRef(); ref == nil || ref.Kind != "Broker" {
		return eventTypes, nil
	}

	for _, et := range source.Spec.EventTypes {
		args := &resources.EventTypeArgs{
			Type:   sourcesv1alpha1.GitHubEventType(et),
			Source: sourcesv1alpha1.GitHubEventSource(source.Spec.OwnerAndRepository),
			Src:    source,
		}
		eventType := resources.MakeEventType(args)
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *sourcesv1alpha1.GitHubSource) ([]eventingv1alpha1.EventType, error) {
	etl, err := r.eventTypeLister.EventTypes(src.Namespace).List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, et := range etl {
		if metav1.IsControlledBy(et, src) {
			eventTypes = append(eventTypes, *et)
		}
	}
	return eventTypes, nil
}

func (r *Reconciler) computeDiff(current []eventingv1alpha1.EventType, expected []eventingv1alpha1.EventType) ([]eventingv1alpha1.EventType, []eventingv1alpha1.EventType) {
	toCreate := make([]eventingv1alpha1.EventType, 0)
	toDelete := make([]eventingv1alpha1.EventType, 0)
	currentMap := asMap(current, keyFromEventType)
	expectedMap := asMap(expected, keyFromEventType)

	// Iterate over the slices instead of the maps for predictable UT expectations.
	for _, e := range expected {
		if c, ok := currentMap[keyFromEventType(&e)]; !ok {
			toCreate = append(toCreate, e)
		} else {
			if !equality.Semantic.DeepEqual(e.Spec, c.Spec) {
				toDelete = append(toDelete, c)
				toCreate = append(toCreate, e)
			}
		}
	}
	// Need to check whether the current EventTypes are not in the expected map. If so, we have to delete them.
	// This could happen if the GitHubSource CO changes its broker.
	for _, c := range current {
		if _, ok := expectedMap[keyFromEventType(&c)]; !ok {
			toDelete = append(toDelete, c)
		}
	}
	return toCreate, toDelete
}

func asMap(eventTypes []eventingv1alpha1.EventType, keyFunc func(*eventingv1alpha1.EventType) string) map[string]eventingv1alpha1.EventType {
	eventTypesAsMap := make(map[string]eventingv1alpha1.EventType, 0)
	for _, eventType := range eventTypes {
		key := keyFunc(&eventType)
		eventTypesAsMap[key] = eventType
	}
	return eventTypesAsMap
}

func keyFromEventType(eventType *eventingv1alpha1.EventType) string {
	return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
}
