/*
Copyright 2019 The Knative Authors

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

package reconciler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing-contrib/prometheus/pkg/reconciler/resources"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-contrib/prometheus/pkg/apis/sources/v1alpha1"
	versioned "knative.dev/eventing-contrib/prometheus/pkg/client/clientset/versioned"
	listers "knative.dev/eventing-contrib/prometheus/pkg/client/listers/sources/v1alpha1"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	prometheussourceReconciled         = "PrometheusSourceReconciled"
	prometheussourceReadinessChanged   = "PrometheusSourceReadinessChanged"
	prometheussourceUpdateStatusFailed = "PrometheusSourceUpdateStatusFailed"
	prometheussourceDeploymentCreated  = "PrometheusSourceDeploymentCreated"
	prometheussourceDeploymentUpdated  = "PrometheusSourceDeploymentUpdated"
)

var (
	deploymentGVK        = appsv1.SchemeGroupVersion.WithKind("Deployment")
	prometheusEventTypes = []string{
		v1alpha1.PromQLPrometheusSourceEventType,
	}
)

type envConfig struct {
	Image string `envconfig:"PROMETHEUS_RA_IMAGE" required:"true"`
}

// Reconciler reconciles a PrometheusSource object
type Reconciler struct {
	*reconciler.Base

	receiveAdapterImage string

	// listers index properties about resources
	prometheussourceLister listers.PrometheusSourceLister
	deploymentLister       appsv1listers.DeploymentLister
	eventTypeLister        eventinglisters.EventTypeLister

	prometheusClientSet versioned.Interface

	sinkResolver *resolver.URIResolver
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PrometheusSource
// resource with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Get the PrometheusSource resource with this namespace/name
	original, err := r.prometheussourceLister.PrometheusSources(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("PrometheusSource key in work queue no longer exists", zap.Any("key", key))
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	prometheussource := original.DeepCopy()

	// Reconcile this copy of the PrometheusSource and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, prometheussource)
	if err != nil {
		logging.FromContext(ctx).Warn("Error reconciling PrometheusSource", zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug("PrometheusSource reconciled")
		r.Recorder.Eventf(prometheussource, corev1.EventTypeNormal, prometheussourceReconciled, `PrometheusDbSource reconciled: "%s/%s"`, prometheussource.Namespace, prometheussource.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, prometheussource.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the PrometheusSource", zap.Error(err))
		r.Recorder.Eventf(prometheussource, corev1.EventTypeWarning, prometheussourceUpdateStatusFailed, "Failed to update PrometheusSource's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return err
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha1.PrometheusSource) error {
	source.Status.InitializeConditions()

	if source.Spec.Sink == nil {
		source.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

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

	sinkURI, err := r.sinkResolver.URIFromDestination(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	if source.Spec.Sink.DeprecatedAPIVersion != "" &&
		source.Spec.Sink.DeprecatedKind != "" &&
		source.Spec.Sink.DeprecatedName != "" {
		source.Status.MarkSinkWarnRefDeprecated(sinkURI)
	} else {
		source.Status.MarkSink(sinkURI)
	}
	source.Status.MarkSink(sinkURI)

	_, err = cron.ParseStandard(source.Spec.Schedule)
	if err != nil {
		source.Status.MarkInvalidSchedule("Invalid", "Reason: "+err.Error())
		return fmt.Errorf("invalid schedule: %v", err)
	}
	source.Status.MarkValidSchedule()

	ra, err := r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		r.Logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	// Update source status// Update source status
	source.Status.PropagateDeploymentAvailability(ra)

	err = r.reconcileEventTypes(ctx, source)
	if err != nil {
		source.Status.MarkNoEventTypes("EventTypesReconcileFailed", "")
		return err
	}
	source.Status.MarkEventTypes()

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.PrometheusSource, sinkURI string) (*appsv1.Deployment, error) {
	eventSource, err := r.makeEventSource(src)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debug("event source", zap.Any("source", eventSource))

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		r.Logger.Panicf("required environment variable is not defined: %v", err)
	}

	adapterArgs := resources.ReceiveAdapterArgs{
		EventSource: eventSource,
		Image:       env.Image,
		Source:      src,
		Labels:      resources.Labels(src.Name),
		SinkURI:     sinkURI,
	}
	expected := resources.MakeReceiveAdapter(&adapterArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		r.Recorder.Eventf(src, corev1.EventTypeNormal, prometheussourceDeploymentCreated, "Deployment created, error: %v", err)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by PrometheusSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, prometheussourceDeploymentUpdated, "Deployment updated")
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *v1alpha1.PrometheusSource) error {
	current, err := r.getEventTypes(ctx, src)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get existing event types", zap.Error(err))
		return err
	}

	expected, err := r.makeEventTypes(src)
	if err != nil {
		return err
	}

	toCreate, toDelete := r.computeDiff(current, expected)

	for _, eventType := range toDelete {
		if err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Delete(eventType.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("Error deleting eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	for _, eventType := range toCreate {
		if _, err = r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).Create(&eventType); err != nil {
			logging.FromContext(ctx).Error("Error creating eventType", zap.Any("eventType", eventType))
			return err
		}
	}

	return err
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *v1alpha1.PrometheusSource) ([]eventingv1alpha1.EventType, error) {
	etl, err := r.EventingClientSet.EventingV1alpha1().EventTypes(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
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

func (r *Reconciler) makeEventTypes(src *v1alpha1.PrometheusSource) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)

	// Only create EventTypes for Broker sinks.
	// We add this check here in case the PrometheusSource was changed from Broker to non-Broker sink.
	// If so, we need to delete the existing ones, thus we return empty expected.
	if ref := src.Spec.Sink.GetRef(); ref == nil || ref.Kind != "Broker" {
		return eventTypes, nil
	}

	source, err := r.makeEventSource(src)
	if err != nil {
		return nil, err
	}

	args := &resources.EventTypeArgs{
		Src:    src,
		Source: source,
	}
	for _, apiEventType := range prometheusEventTypes {
		args.Type = apiEventType
		eventType := resources.MakeEventType(args)
		eventTypes = append(eventTypes, eventType)
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
	// This could happen if the PrometheusSource CO changes its broker.
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.PrometheusSource) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getLabelSelector(src *v1alpha1.PrometheusSource) labels.Selector {
	return labels.SelectorFromSet(resources.Labels(src.Name))
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.PrometheusSource) (*v1alpha1.PrometheusSource, error) {
	prometheussource, err := r.prometheussourceLister.PrometheusSources(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(prometheussource.Status, desired.Status) {
		return prometheussource, nil
	}

	becomesReady := desired.Status.IsReady() && !prometheussource.Status.IsReady()

	// Don't modify the informers copy.
	existing := prometheussource.DeepCopy()
	existing.Status = desired.Status

	cj, err := r.prometheusClientSet.SourcesV1alpha1().PrometheusSources(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(cj.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("PrometheusSource %q became ready after %v", prometheussource.Name, duration)
		r.Recorder.Event(prometheussource, corev1.EventTypeNormal, prometheussourceReadinessChanged, fmt.Sprintf("PrometheusSource %q became ready", prometheussource.Name))
		if err := r.StatsReporter.ReportReady("PrometheusSource", prometheussource.Namespace, prometheussource.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for PrometheusSource, %v", err)
		}
	}

	return cj, err
}

// MakeEventSource computes the Cloud Event source attribute for the given source
func (r *Reconciler) makeEventSource(src *v1alpha1.PrometheusSource) (string, error) {
	return fmt.Sprintf("/apis/v1/namespaces/%s/prometheussources/%s#%s?%s",
		src.Namespace, src.Name, src.Spec.ServerURL, src.Spec.PromQL), nil
}
