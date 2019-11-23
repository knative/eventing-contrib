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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"

	//k8s.io imports
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	//knative.dev/serving imports
	servingv1alpha1 "knative.dev/serving/pkg/apis/serving/v1alpha1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1alpha1"

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
	gitHubSourceReadinessChanged   = "GitHubSourceReadinessChanged"
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

	githubClientSet clientset.Interface
	githubLister    listers.GitHubSourceLister

	servingClientSet servingclientset.Interface
	servingLister    servinglisters.ServiceLister

	receiveAdapterImage string
	eventTypeLister     eventinglisters.EventTypeLister

	deploymentLister appsv1listers.DeploymentLister
	sinkResolver     *resolver.URIResolver

	loggingContext context.Context
	loggingConfig  *logging.Config

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
			ksvcArgs := resources.ServiceArgs{
				Source:              source,
				ReceiveAdapterImage: r.receiveAdapterImage,
			}
			ksvc := resources.MakeService(&ksvcArgs)
			if _, err := r.servingClientSet.ServingV1alpha1().Services(source.Namespace).Create(ksvc); err != nil {
				return err
			}
			r.Recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
			// TODO: Mark Deploying for the ksvc
			// Wait for the Service to get a status
			return nil
		} else if !metav1.IsControlledBy(ksvc, source) {
			return fmt.Errorf("Service %q is not owned by GitHubSource %q", ksvc.Name, source.Name)
		}
		// Error was something other than NotFound
		return err
	}

	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionReady)
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && ksvc.Status.URL != nil {
		receiveAdapterDomain := ksvc.Status.URL.Host
		// source.Status.MarkServiceDeployed(ra)
		// TODO: Mark Deployed for the ksvc
		// TODO: Mark some condition for the webhook status?
		err := r.addFinalizer(source)
		if err != nil {
			return err
		}
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
	accessor, err := meta.Accessor(source)
	if accessor.GetDeletionTimestamp() != nil {
		err = r.finalize(ctx, source)
		if err != nil {
			return err
		}
	}

	err = r.reconcileEventTypes(ctx, source)
	if err != nil {
		// should we mark no event types here in stat
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
	_, err := r.githubClientSet.SourcesV1alpha1().GitHubSources(source.Namespace).Update(source)
	if err != nil {
		return err
	}

	// If a webhook was created, try to delete it
	if source.Status.WebhookIDKey != "" {
		// Get access token
		accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
			r.Recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
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

func (r *Reconciler) getOwnedService(ctx context.Context, source *sourcesv1alpha1.GitHubSource) (*servingv1alpha1.Service, error) {
	listOptions := &metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	}
	serviceList, err := r.servingClientSet.ServingV1alpha1().Services(source.Namespace).List(*listOptions)
	if err != nil {
		return nil, err
	}
	for _, ksvc := range serviceList.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			//TODO if there are >1 controlled, delete all but first?
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
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
	eventTypes := make([]eventingv1alpha1.EventType, 0)

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

func (r *Reconciler) addFinalizer(src *sourcesv1alpha1.GitHubSource) error {
	finalizers := sets.NewString(src.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(src.Finalizers, finalizerName),
			"resourceVersion": src.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return nil
	}
	_, err = r.githubClientSet.SourcesV1alpha1().GitHubSources(src.Namespace).Patch(src.Name, types.MergePatchType, patch)
	return err
}

func (r *Reconciler) removeFinalizer(s *sourcesv1alpha1.GitHubSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
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
		r.Recorder.Event(github, corev1.EventTypeNormal, gitHubSourceReadinessChanged, fmt.Sprintf("GitHubSource %q became ready", github.Name))
	}
	return result, err
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *sourcesv1alpha1.GitHubSource) ([]eventingv1alpha1.EventType, error) {
	etl, err := r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).List(metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list event types: %v", zap.Error(err))
		return nil, err
	}
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, et := range etl.Items {
		if metav1.IsControlledBy(&et, src) {
			eventTypes = append(eventTypes, et)
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

func (r *Reconciler) podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
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
