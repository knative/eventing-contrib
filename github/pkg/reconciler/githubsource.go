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
	//	"log"
	//	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	//k8s.io imports
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	//	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	//	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	//	"k8s.io/client-go/tools/record"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	//	"sigs.k8s.io/controller-runtime/pkg/manager"

	//knative.dev/eventing-contrib imports
	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	clientset "knative.dev/eventing-contrib/github/pkg/client/clientset/versioned"
	listers "knative.dev/eventing-contrib/github/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing-contrib/github/pkg/reconciler/resources"

	//knative.dev/eventing imports
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"

	//knative.dev/pkg imports
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName            = "github-source-controller"
	raImageEnvVar                  = "GH_RA_IMAGE"
	finalizerName                  = controllerAgentName
	gitHubSourceUpdateStatusFailed = "GitHubSourceUpdateStatusFailed"
	gitHubSourceDeploymentCreated  = "GitHubSourceDeploymentCreated"
	gitHubSourceDeploymentUpdated  = "GitHubSourceDeploymentUpdated"
	gitHubSourceReconciled         = "GitHubSourceReconciled"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

// Reconciler reconciles a GitHubSource object
type Reconciler struct {
	*reconciler.Base

	githubClientSet     clientset.Interface
	receiveAdapterImage string
	eventTypeLister     eventinglisters.EventTypeLister
	githubLister        listers.GitHubSourceLister
	sinkResolver        *resolver.URIResolver
	deploymentLister    appsv1listers.DeploymentLister
	loggingContext      context.Context
	loggingConfig       *logging.Config

	webhookClient webhookClient
}

type webhookArgs struct {
	source                *sourcesv1alpha1.GitHubSource
	domain                string
	accessToken           string
	secretToken           string
	alternateGitHubAPIURL string
	hookID                string
}

// Reconcile reads that state of the cluster for a GitHubSource
// object and makes changes based on the state read and what is in the
// GitHubSource.Spec
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Error("invalid resource key: %s", zap.Any("key", key))
	}
	source, ok := r.githubLister.GitHubSources(namespace).Get(name)
	if apierrors.IsNotFound(ok) {
		logger.Error("could not find github source", zap.Any("key", key))
		return nil
	} else if ok != nil {
		return err
	}

	// Don't modify the original (informers) copy
	github := source.DeepCopy()

	// Reconcile this copy of the Subscription and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, github)
	if reconcileErr != nil {
		logging.FromContext(ctx).Warn("Error reconciling GitHubSource", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("GitHubSource reconciled")
		r.Recorder.Eventf(github, corev1.EventTypeNormal, gitHubSourceReconciled, "GitHubSource reconciled: %q", github.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, github.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the GitHubSource", zap.Error(updateStatusErr))
		r.Recorder.Eventf(github, corev1.EventTypeWarning, gitHubSourceUpdateStatusFailed, "Failed to update GitHubSource's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.GitHubSource) error {
	logger := logging.FromContext(ctx)

	source.Status.ObservedGeneration = source.Generation
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
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc = resources.MakeService(source, r.receiveAdapterImage)
			if err = controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
				return err
			}
			if err = r.KubeClientSet.AppsV1().Deployments(source.Namespace).Create(ksvc); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
			// TODO: Mark Deploying for the ksvc
			// Wait for the Service to get a status
			return nil
		}
		// Error was something other than NotFound
		return err
	}

	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionReady)
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && ksvc.Status.URL != nil {
		receiveAdapterDomain := ksvc.Status.URL.Host
		// TODO: Mark Deployed for the ksvc
		// TODO: Mark some condition for the webhook status?
		r.addFinalizer(source)
		if source.Status.WebhookIDKey == "" {
			args := &webhookArgs{
				source:                source,
				domain:                receiveAdapterDomain,
				accessToken:           accessToken,
				secretToken:           secretToken,
				alternateGitHubAPIURL: source.Spec.GitHubAPIURL,
			}

			hookID, err := r.createWebhook(ctx, args)
			if err != nil {
				return err
			}
			source.Status.WebhookIDKey = hookID
		}
	} else {
		return nil
	}

	err = r.reconcileEventTypes(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkEventTypes()

	return nil
}

func (r *Reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.GitHubSource) error {
	// Always remove the finalizer. If there's a failure cleaning up, an event
	// will be recorded allowing the webhook to be removed manually by the
	// operator.
	r.removeFinalizer(source)

	// If a webhook was created, try to delete it
	if source.Status.WebhookIDKey != "" {
		// Get access token
		accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
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
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
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

	hookOptions := &webhookOptions{
		accessToken: args.accessToken,
		secretToken: args.secretToken,
		domain:      args.domain,
		owner:       owner,
		repo:        repo,
		events:      args.source.Spec.EventTypes,
		secure:      args.source.Spec.Secure,
	}
	hookID, err := r.webhookClient.Create(ctx, hookOptions, args.alternateGitHubAPIURL)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook: %v", err)
	}
	return hookID, nil
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
		secure:      args.source.Spec.Secure,
	}
	err = r.webhookClient.Delete(ctx, hookOptions, args.hookID, args.alternateGitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %v", err)
	}
	return nil
}

func (r *Reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	err := r.githubClientSet.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)
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

func (r *Reconciler) getOwnedService(ctx context.Context, source *sourcesv1alpha1.GitHubSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	lo := &client.ListOptions{
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
	}
	err := r.githubClientSet.List(ctx, list, lo)
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

func (r *Reconciler) reconcileEventTypes(ctx context.Context, source *sourcesv1alpha1.GitHubSource) error {
	args := r.newEventTypeReconcilerArgs(source)
	return r.eventTypeReconciler.Reconcile(ctx, source, args)
}

func (r *Reconciler) newEventTypeReconcilerArgs(source *sourcesv1alpha1.GitHubSource) *eventtype.ReconcilerArgs {
	specs := make([]eventingv1alpha1.EventTypeSpec, 0)
	for _, et := range source.Spec.EventTypes {
		spec := eventingv1alpha1.EventTypeSpec{
			Type:   sourcesv1alpha1.GitHubEventType(et),
			Source: sourcesv1alpha1.GitHubEventSource(source.Spec.OwnerAndRepository),
			Broker: source.Spec.Sink.Name,
		}
		specs = append(specs, spec)
	}
	return &eventtype.ReconcilerArgs{
		Specs:     specs,
		Namespace: source.Namespace,
		Labels:    resources.Labels(source.Name),
		Kind:      source.Spec.Sink.Kind,
	}
}

func (r *Reconciler) addFinalizer(s *sourcesv1alpha1.GitHubSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) removeFinalizer(s *sourcesv1alpha1.GitHubSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.client = c
	r.eventTypeReconciler.Client = c
	return nil
}

func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		logging.FromContext(r.loggingContext).Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	logging.FromContext(r.loggingContext).Info("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) updateStatus(ctx context.Context, source *sourcesv1alpha1.GitHubSource) (*sourcesv1alpha1.GitHubSource, error) {
	github, err := r.githubLister.GitHubSources(source.Namespace).Get(source.Name)
	if err != nil {
		return nil, err
	}

	// If the status remains the same (nothing to update), return.
	if reflect.DeepEqual(github.Status, source.Status) {
		return github, nil
	}

	becomesReady := source.Status.IsReady() && !github.Status.IsReady()

	// We avoid modifying the informer's copy
	existing := github.DeepCopy()
	existing.Status = source.Status

	result, err := r.githubClientSet.SourcesV1alpha1().GitHubSources(source.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(result.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Info("GitHubSource became ready after", zap.Duration("duration", duration))
		r.Recorder.Event(github, corev1.EventTypeNormal, GitHubReadinessChanged, fmt.Sprintf("GitHubSource %q became ready", github.Name))
	}
	return result, err
}
