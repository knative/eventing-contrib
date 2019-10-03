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

package kafka

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/kafka/source/pkg/reconciler/resources"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	"knative.dev/pkg/logging"

	// NewController stuff
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	clientset "knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
	listers "knative.dev/eventing-contrib/kafka/source/pkg/client/listers/sources/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"

	"knative.dev/pkg/controller"
	pkgLogging "knative.dev/pkg/logging"
)

const (
	raImageEnvVar                = "KAFKA_RA_IMAGE"
	kafkaReadinessChanged        = "KafkaSourceReadinessChanged"
	kafkaUpdateStatusFailed      = "KafkaSourceUpdateStatusFailed"
	kafkaSourceDeploymentCreated = "KafkaSourceDeploymentCreated"
	kafkaSourceDeploymentUpdated = "KafkaSourceDeploymentUpdated"
	kafkaSourceReconciled        = "KafkaSourceReconciled"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

type Reconciler struct {
	*reconciler.Base

	kafkaClientSet      clientset.Interface
	receiveAdapterImage string
	eventTypeLister     eventinglisters.EventTypeLister
	kafkaLister         listers.KafkaSourceLister
	sinkReconciler      *duck.SinkReconciler
	deploymentLister    appsv1listers.DeploymentLister
	loggingContext      context.Context
	loggingConfig       *pkgLogging.Config
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx).Desugar()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Error("invalid resource key: %s", zap.Any("key", key))
		return nil
	}

	src, ok := r.kafkaLister.KafkaSources(namespace).Get(name)
	if apierrs.IsNotFound(ok) {
		logger.Error("could not find Apache Kafka source", zap.Any("key", key))
		return nil
	} else if ok != nil {
		return err
	}
	kafka := src.DeepCopy()
	err = r.reconcile(ctx, kafka)

	if err != nil {
		logger.Warn("Error Reconciling KafkaSource", zap.Error(err))
	} else {
		logger.Debug("KafkaSource reconciled")
		r.Recorder.Eventf(kafka, corev1.EventTypeNormal, kafkaSourceReconciled, "`KafkaSource reconciled: %s/%s", kafka.Namespace, kafka.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, kafka.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the KafkaSource", zap.Error(err))
		r.Recorder.Eventf(kafka, corev1.EventTypeWarning, kafkaUpdateStatusFailed, "Failed to update KafkaSource's status: %v", err)
		return updateStatusErr
	}

	return err
}

func (r *Reconciler) reconcile(ctx context.Context, src *v1alpha1.KafkaSource) error {
	logger := logging.FromContext(ctx)

	src.Status.ObservedGeneration = src.Generation
	src.Status.InitializeConditions()

	if src.Spec.Sink == nil {
		src.Status.MarkNoSink("Missing", "Sink missing from spec")
		return errors.New("spec.sink missing")
	}

	sinkObjRef := src.Spec.Sink
	if sinkObjRef.Namespace == "" {
		sinkObjRef.Namespace = src.Namespace
	}
	kafkaDesc := fmt.Sprintf("%s/%s,%s", src.Namespace, src.Name, src.GroupVersionKind().String())
	sinkURI, err := r.sinkReconciler.GetSinkURI(sinkObjRef, src, kafkaDesc)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}

	src.Status.MarkSink(sinkURI)

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	src.Status.MarkDeployed(ra)

	err = r.reconcileEventTypes(ctx, src)
	if err != nil {
		src.Status.MarkNoEventTypes("EventTypesreconcileFailed", "")
		logger.Error("Unable to reconcile the event types", zap.Error(err))
		return err
	}
	src.Status.MarkEventTypes()

	return nil
}

func checkResourcesStatus(src *v1alpha1.KafkaSource) error {

	for _, rsrc := range []struct {
		key   string
		field string
	}{{
		key:   "Request.CPU",
		field: src.Spec.Resources.Requests.ResourceCPU,
	}, {
		key:   "Request.Memory",
		field: src.Spec.Resources.Requests.ResourceMemory,
	}, {
		key:   "Limit.CPU",
		field: src.Spec.Resources.Limits.ResourceCPU,
	}, {
		key:   "Limit.Memory",
		field: src.Spec.Resources.Limits.ResourceMemory,
	}} {
		// In the event the field isn't specified, we assign a default in the receive_adapter
		if rsrc.field != "" {
			if _, err := resource.ParseQuantity(rsrc.field); err != nil {
				src.Status.MarkResourcesIncorrect("Incorrect Resource", "%s: %s, Error: %s", rsrc.key, rsrc.field, err)
				return err
			}
		}
	}
	src.Status.MarkResourcesCorrect()
	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.KafkaSource, sinkURI string) (*v1.Deployment, error) {

	if err := checkResourcesStatus(src); err != nil {
		return nil, err
	}

	loggingConfig, err := pkgLogging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting logging config to JSON", zap.Any("receiveAdapter", err))
	}
	raArgs := resources.ReceiveAdapterArgs{
		Image:         r.receiveAdapterImage,
		Source:        src,
		Labels:        resources.GetLabels(src.Name),
		LoggingConfig: loggingConfig,
		SinkURI:       sinkURI,
	}
	expected := resources.MakeReceiveAdapter(&raArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		msg := "Deployment created"
		if err != nil {
			msg = fmt.Sprintf("Deployment created, error: %v", err)
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, kafkaSourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by KafkaSource %q", ra.Name, src.Name)
	} else if podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, kafkaSourceDeploymentUpdated, "Deployment updated")
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.KafkaSource) (*v1.Deployment, error) {
	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range ra.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) reconcileEventTypes(ctx context.Context, src *v1alpha1.KafkaSource) error {
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
	return nil
}

func (r *Reconciler) getEventTypes(ctx context.Context, src *v1alpha1.KafkaSource) ([]eventingv1alpha1.EventType, error) {
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

func (r *Reconciler) getLabelSelector(src *v1alpha1.KafkaSource) labels.Selector {
	return labels.SelectorFromSet(resources.GetLabels(src.Name))
}

func (r *Reconciler) updateStatus(ctx context.Context, src *v1alpha1.KafkaSource) (*v1alpha1.KafkaSource, error) {
	kafka, err := r.kafkaLister.KafkaSources(src.Namespace).Get(src.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(kafka.Status, src.Status) {
		return kafka, nil
	}

	becomesReady := src.Status.IsReady() && !kafka.Status.IsReady()

	// Don't modify the informers copy.
	existing := kafka.DeepCopy()
	existing.Status = src.Status

	result, err := r.kafkaClientSet.SourcesV1alpha1().KafkaSources(src.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(result.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Info("KafkaSource became ready after", zap.Duration("duration", duration))
		r.Recorder.Event(kafka, corev1.EventTypeNormal, kafkaReadinessChanged, fmt.Sprintf("KafkaSource %q became ready", kafka.Name))
	}
	return result, err
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
	// This could happen if the KafkaSource CO changes its broker.
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

func (r *Reconciler) makeEventTypes(src *v1alpha1.KafkaSource) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	// Only create EventTypes for Broker sinks.
	// We add this check here in case the KafkaSource was changed from Broker to non-Broker sink.
	// If so, we need to delete the existing ones, thus we return empty expected.
	if src.Spec.Sink.Kind != "Broker" {
		return eventTypes, nil
	}
	topics := strings.Split(src.Spec.Topics, ",")
	for _, topic := range topics {
		args := &resources.EventTypeArgs{
			Src:    src,
			Type:   v1alpha1.KafkaEventType,
			Source: v1alpha1.KafkaEventSource(src.Namespace, src.Name, topic),
		}
		eventType := resources.MakeEventType(args)
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := pkgLogging.NewConfigFromConfigMap(cfg)
	if err != nil {
		logging.FromContext(r.loggingContext).Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	logging.FromContext(r.loggingContext).Info("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
}
