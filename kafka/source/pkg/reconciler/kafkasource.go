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
	"strings"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/metrics"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/kafka/source/pkg/reconciler/resources"

	// NewController stuff
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
	reconcilerkafkasource "knative.dev/eventing-contrib/kafka/source/pkg/client/injection/reconciler/sources/v1alpha1/kafkasource"
	listers "knative.dev/eventing-contrib/kafka/source/pkg/client/listers/sources/v1alpha1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	raImageEnvVar                = "KAFKA_RA_IMAGE"
	kafkaReadinessChanged        = "KafkaSourceReadinessChanged"
	kafkaUpdateStatusFailed      = "KafkaSourceUpdateStatusFailed"
	kafkaSourceDeploymentCreated = "KafkaSourceDeploymentCreated"
	kafkaSourceDeploymentUpdated = "KafkaSourceDeploymentUpdated"
	kafkaSourceDeploymentFailed  = "KafkaSourceDeploymentUpdated"
	kafkaSourceReconciled        = "KafkaSourceReconciled"
	component                    = "kafkasource"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

// newDeploymentCreated makes a new reconciler event with event type Normal, and
// reason KafkaSourceDeploymentCreated.
func newDeploymentCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, kafkaSourceDeploymentCreated, "KafkaSource created deployment: \"%s/%s\"", namespace, name)
}

// deploymentUpdated makes a new reconciler event with event type Normal, and
// reason KafkaSourceDeploymentUpdated.
func deploymentUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, kafkaSourceDeploymentUpdated, "KafkaSource updated deployment: \"%s/%s\"", namespace, name)
}

// newDeploymentFailed makes a new reconciler event with event type Warning, and
// reason KafkaSourceDeploymentFailed.
func newDeploymentFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, kafkaSourceDeploymentFailed, "KafkaSource failed to create deployment: \"%s/%s\", %w", namespace, name, err)
}

type Reconciler struct {
	// KubeClientSet allows us to talk to the k8s for core APIs
	KubeClientSet kubernetes.Interface

	receiveAdapterImage string

	kafkaLister      listers.KafkaSourceLister
	deploymentLister appsv1listers.DeploymentLister

	kafkaClientSet versioned.Interface
	loggingContext context.Context
	loggingConfig  *logging.Config
	metricsConfig  *metrics.ExporterOptions

	sinkResolver *resolver.URIResolver
}

// Check that our Reconciler implements Interface
var _ reconcilerkafkasource.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.KafkaSource) pkgreconciler.Event {
	src.Status.InitializeConditions()

	if src.Spec.Sink == nil {
		src.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

	dest := src.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if dest.Ref.Namespace == "" {
			dest.Ref.Namespace = src.GetNamespace()
		}
	}
	sinkURI, err := r.sinkResolver.URIFromDestinationV1(*dest, src)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}
	src.Status.MarkSink(sinkURI)

	if val, ok := src.GetLabels()[v1alpha1.KafkaKeyTypeLabel]; ok {
		found := false
		for _, allowed := range v1alpha1.KafkaKeyTypeAllowed {
			if allowed == val {
				found = true
			}
		}
		if found == false {
			src.Status.MarkResourcesIncorrect("IncorrectKafkaKeyTypeLabel", "Invalid value for %s: %s. Allowed: %v", v1alpha1.KafkaKeyTypeLabel, val, v1alpha1.KafkaKeyTypeAllowed)
			logging.FromContext(ctx).Errorf("Invalid value for %s: %s. Allowed: %v", v1alpha1.KafkaKeyTypeLabel, val, v1alpha1.KafkaKeyTypeAllowed)
			return errors.New("IncorrectKafkaKeyTypeLabel")
		}
	}

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	src.Status.MarkDeployed(ra)
	src.Status.CloudEventAttributes = r.createCloudEventAttributes(src)

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

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.KafkaSource, sinkURI *apis.URL) (*appsv1.Deployment, error) {

	if err := checkResourcesStatus(src); err != nil {
		return nil, err
	}

	loggingConfig, err := logging.LoggingConfigToJson(r.loggingConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting logging config to JSON", zap.Any("receiveAdapter", err))
	}
	metricsConfig, err := metrics.MetricsOptionsToJson(r.metricsConfig)
	if err != nil {
		logging.FromContext(ctx).Error("error while converting metrics config to JSON", zap.Any("receiveAdapter", err))
	}

	raArgs := resources.ReceiveAdapterArgs{
		Image:         r.receiveAdapterImage,
		Source:        src,
		Labels:        resources.GetLabels(src.Name),
		LoggingConfig: loggingConfig,
		MetricsConfig: metricsConfig,
		SinkURI:       sinkURI.String(),
	}
	expected := resources.MakeReceiveAdapter(&raArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		if err != nil {
			return nil, newDeploymentFailed(ra.Namespace, ra.Name, err)
		}
		return ra, newDeploymentCreated(ra.Namespace, ra.Name)
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
		return ra, deploymentUpdated(ra.Namespace, ra.Name)
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.KafkaSource) (*appsv1.Deployment, error) {
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

func (r *Reconciler) getLabelSelector(src *v1alpha1.KafkaSource) labels.Selector {
	return labels.SelectorFromSet(resources.GetLabels(src.Name))
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

func (r *Reconciler) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	r.metricsConfig = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		Component: component,
		ConfigMap: cfg.Data,
	}
	logging.FromContext(r.loggingContext).Info("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) createCloudEventAttributes(src *v1alpha1.KafkaSource) []duckv1.CloudEventAttributes {
	topics := strings.Split(src.Spec.Topics, ",")
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(topics))
	for _, topic := range topics {
		ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
			Type:   v1alpha1.KafkaEventType,
			Source: v1alpha1.KafkaEventSource(src.Namespace, src.Name, topic),
		})
	}
	return ceAttributes
}
