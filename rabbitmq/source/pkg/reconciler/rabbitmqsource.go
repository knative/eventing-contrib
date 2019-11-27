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

package rabbitmq

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing-contrib/rabbitmq/source/pkg/client/clientset/versioned"
	"knative.dev/eventing-contrib/rabbitmq/source/pkg/apis/sources/v1alpha1"
	reconcilerrabbitmqsource "knative.dev/eventing-contrib/rabbitmq/source/pkg/client/injection/reconciler/sources/v1alpha1/rabbitmqsource"
	listers "knative.dev/eventing-contrib/rabbitmq/source/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing-contrib/rabbitmq/source/pkg/reconciler/resources"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

const (
	raImageEnvVar              		= "RABBITMQ_RA_IMAGE"
	rabbitmqReadinessChanged   		= "RabbitmqSourceReadinessChanged"
	rabbitmqUpdateStatusFailed      = "RabbitmqSourceUpdateStatusFailed"
	rabbitmqSourceDeploymentCreated = "RabbitmqSourceDeploymentCreated"
	rabbitmqSourceDeploymentUpdated = "RabbitmqSourceDeploymentUpdated"
	rabbitmqSourceDeploymentFailed  = "RabbitmqSourceDeploymentUpdated"
	rabbitmqSourceReconciled        = "RabbitmqSourceReconciled"
	component       				= "rabbitmqsource"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
)

func newDeploymentCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, rabbitmqSourceDeploymentCreated, "RabbitmqSource created deployment: \"%s/%s\"", namespace, name)
}

func deploymentUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, rabbitmqSourceDeploymentUpdated, "RabbitmqSource updated deployment: \"%s/%s\"", namespace, name)
}

func newDeploymentFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, rabbitmqSourceDeploymentFailed, "RabbitmqSource failed to create deployment: \"%s/%s\", %w", namespace, name, err)
}

type Reconciler struct {
	// KubeClientSet allows us to talk to the k8s for core APIs
	KubeClientSet kubernetes.Interface

	receiveAdapterImage string

	rabbitmqLister	 listers.RabbitmqSourceLister
	deploymentLister appsv1listers.DeploymentLister

	rabbitmqClientSet   versioned.Interface
	loggingContext		context.Context
	loggingConfig 		*pkgLogging.Config
	metricsConfig 		*metrics.ExporterOptions

	sinkResolver		*resolver.URIResolver
}

var _ reconcilerrabbitmqsource.Interface  = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.RabbitmqSource) pkgreconciler.Event {
	src.Status.InitializeConditions()

	if src.Spec.Sink == nil {
		src.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

	dest := src.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
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

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	src.Status.MarkDeployed(ra)
	src.Status.CloudEventAttributes = r.createCloudEventAttributes(src)

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.RabbitmqSource, sinkURI *apis.URL) (*v1.Deployment, error) {

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
		SinkURI:       sinkURI.String(),
		MetricsConfig: metricsConfig,
		LoggingConfig: loggingConfig,
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
		return nil, fmt.Errorf("deployment %q is not owned by RabbitmqSource %q", ra.Name, src.Name)
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.RabbitmqSource) (*v1.Deployment, error) {
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

func (r *Reconciler) getLabelSelector(src *v1alpha1.RabbitmqSource) labels.Selector {
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

func (r *Reconciler) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap)  {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	r.metricsConfig = &metrics.ExporterOptions{
		Domain: metrics.Domain(),
		Component: component,
		ConfigMap: cfg.Data,
	}
	logging.FromContext(r.loggingContext).Info("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) createCloudEventAttributes(src *v1alpha1.RabbitmqSource) []duckv1.CloudEventAttributes {
	ceAttribute := duckv1.CloudEventAttributes{
		Type:   v1alpha1.RabbitmqEventType,
		Source: v1alpha1.RabbitmqEventSource(src.Namespace, src.Name, src.Spec.Topic),
	}
	return []duckv1.CloudEventAttributes{ceAttribute}
}