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

package githubsource

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	ghclient "github.com/google/go-github/github"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/githubsource/resources"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type webhookCreatorOptions struct {
	accessToken string
	secretToken string
	domain      string
	owner       string
	repo        string
	events      []string
}

// webhookCreator creates a webhook and makes unit testing easier
type webhookCreator func(ctx context.Context, options *webhookCreatorOptions) (string, error)

// gitHubWebhookCreator creates a GitHub webhook
func gitHubWebhookCreator(ctx context.Context, options *webhookCreatorOptions) (string, error) {
	logger := logging.FromContext(ctx)
	domain := options.domain

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: options.accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := ghclient.NewClient(tc)
	active := true
	config := make(map[string]interface{})
	config["url"] = fmt.Sprintf("http://%s", domain)
	config["content_type"] = "json"
	config["secret"] = options.secretToken

	// GitHub hook names are required to be named "web" or the name of a GitHub service
	hookname := "web"
	hook := ghclient.Hook{
		Name:   &hookname,
		URL:    &domain,
		Events: options.events,
		Active: &active,
		Config: config,
	}

	h, resp, err := client.Repositories.CreateHook(ctx, options.owner, options.repo, &hook)
	if err != nil {
		logger.Infof("create webhook error response:\n%+v", resp)
		return "", fmt.Errorf("failed to create the webhook: %v", err)
	}
	logger.Infof("created hook: %+v", h)

	return strconv.FormatInt(*h.ID, 10), nil
}

// reconciler reconciles a GitHubSource object
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	dynamicClient       dynamic.Interface
	recorder            record.EventRecorder
	receiveAdapterImage string
	webhookCreator      webhookCreator
}

// Reconcile reads that state of the cluster for a GitHubSource
// object and makes changes based on the state read and what is in the
// GitHubSource.Spec
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx)

	source, ok := object.(*sourcesv1alpha1.GitHubSource)
	if !ok {
		logger.Errorf("could not find github source %v\n", object)
		return object, nil
	}

	// See if the source has been deleted
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", zap.Error(err))
		return object, err
	}
	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		return object, nil
	}

	reconcileErr := r.reconcile(ctx, source)
	return source, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.GitHubSource) error {
	source.Status.InitializeConditions()

	if source.Spec.Repository == "" {
		source.Status.MarkNotValid("RepositoryMissing", "")
		return errors.New("repository is empty")
	}
	if source.Spec.EventType == "" {
		source.Status.MarkNotValid("EventTypeMissing", "")
		return errors.New("event type is empty")
	}
	if source.Spec.AccessToken.SecretKeyRef == nil {
		source.Status.MarkNotValid("AccessTokenMissing", "")
		return errors.New("access token ref is nil")
	}
	if source.Spec.SecretToken.SecretKeyRef == nil {
		source.Status.MarkNotValid("SecretTokenMissing", "")
		return errors.New("secret token ref is nil")
	}
	source.Status.MarkValid()

	uri, err := sinks.GetSinkURI(r.dynamicClient, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedService(ctx, source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc := resources.MakeService(source, r.receiveAdapterImage)
			if err := controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
				return err
			}
			if err := r.client.Create(ctx, ksvc); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
			// TODO: Mark Deploying for the ksvc
			// Wait for the Service to get a status
			return nil
		}
	} else {
		routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
		receiveAdapterDomain := ksvc.Status.Domain
		if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
			// TODO: Mark Deployed for the ksvc
			// TODO: Mark some condition for the webhook status?
			if source.Status.WebhookIDKey == "" {
				// TODO: We need to handle deleting the webhook if
				// this source gets deleted
				return r.createWebhook(ctx, source, receiveAdapterDomain)
			}
		}
	}

	return nil
}

func (r *reconciler) createWebhook(ctx context.Context, source *sourcesv1alpha1.GitHubSource, domain string) error {
	logger := logging.FromContext(ctx)

	logger.Info("creating GitHub webhook")

	owner, repo, err := parseOwnerRepoFrom(source.Spec.Repository)
	if err != nil {
		return err
	}

	events, err := parseEventsFrom(source.Spec.EventType)
	if err != nil {
		return err
	}

	accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		return fmt.Errorf("failed to get access token from GitHubSource: %v", err)
	}

	secretToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		return fmt.Errorf("failed to get secret token from GitHubSource: %v", err)
	}

	hookOptions := &webhookCreatorOptions{
		accessToken: accessToken,
		secretToken: secretToken,
		domain:      domain,
		owner:       owner,
		repo:        repo,
		events:      events,
	}
	hookID, err := r.webhookCreator(ctx, hookOptions)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %v", err)
	}
	source.Status.WebhookIDKey = hookID
	return nil
}

func (r *reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)
	if err != nil {
		return "", err
	}
	return string(secret.Data[secretKeySelector.Key]), nil
}

func parseOwnerRepoFrom(repository string) (string, string, error) {
	components := strings.Split(repository, "/")
	if len(components) != 2 {
		return "", "", fmt.Errorf("repository is malformatted, expected 'owner/repo' but found %q", repository)
	}
	owner := components[0]
	if len(owner) == 0 {
		return "", "", fmt.Errorf("owner is empty, expected 'owner/repo' but found %q", repository)
	}
	repo := components[1]
	if len(repo) == 0 {
		return "", "", fmt.Errorf("repo is empty, expected 'owner/repo' but found %q", repository)
	}

	return owner, repo, nil
}

func parseEventsFrom(eventType string) ([]string, error) {
	event, ok := sourcesv1alpha1.GitHubSourceGitHubEventType[eventType]
	if !ok {
		return []string(nil), fmt.Errorf("event type is unknown: %s", eventType)
	}
	return []string{event}, nil
}

func (r *reconciler) getOwnedService(ctx context.Context, source *sourcesv1alpha1.GitHubSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: servingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			//TODO if there are >1 controlled, delete all but first?
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}
