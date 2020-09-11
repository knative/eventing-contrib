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

package source

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/reconciler/source"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	"knative.dev/eventing-contrib/kafka/source/pkg/reconciler/source/resources"

	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
	reconcilerkafkasource "knative.dev/eventing-contrib/kafka/source/pkg/client/injection/reconciler/sources/v1beta1/kafkasource"
	listers "knative.dev/eventing-contrib/kafka/source/pkg/client/listers/sources/v1beta1"
)

const (
	raImageEnvVar                = "KAFKA_RA_IMAGE"
	kafkaSourceDeploymentCreated = "KafkaSourceDeploymentCreated"
	kafkaSourceDeploymentUpdated = "KafkaSourceDeploymentUpdated"
	kafkaSourceDeploymentFailed  = "KafkaSourceDeploymentUpdated"
	component                    = "kafkasource"
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

	sinkResolver *resolver.URIResolver

	configs source.ConfigAccessor
}

// Check that our Reconciler implements Interface
var _ reconcilerkafkasource.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1beta1.KafkaSource) pkgreconciler.Event {
	src.Status.InitializeConditions()

	if (src.Spec.Sink == duckv1.Destination{}) {
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
	sinkURI, err := r.sinkResolver.URIFromDestinationV1(ctx, *dest, src)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		//delete adapter deployment if sink not found
		if err := r.deleteReceiveAdapter(ctx, src); err != nil && !apierrors.IsNotFound(err) {
			logging.FromContext(ctx).Error("Unable to delete receiver adapter when sink is missing", zap.Error(err))
		}
		return fmt.Errorf("getting sink URI: %v", err)
	}
	src.Status.MarkSink(sinkURI)

	if val, ok := src.GetLabels()[v1beta1.KafkaKeyTypeLabel]; ok {
		found := false
		for _, allowed := range v1beta1.KafkaKeyTypeAllowed {
			if allowed == val {
				found = true
			}
		}
		if !found {
			src.Status.MarkKeyTypeIncorrect("IncorrectKafkaKeyTypeLabel", "Invalid value for %s: %s. Allowed: %v", v1beta1.KafkaKeyTypeLabel, val, v1beta1.KafkaKeyTypeAllowed)
			logging.FromContext(ctx).Errorf("Invalid value for %s: %s. Allowed: %v", v1beta1.KafkaKeyTypeLabel, val, v1beta1.KafkaKeyTypeAllowed)
			return errors.New("IncorrectKafkaKeyTypeLabel")
		} else {
			src.Status.MarkKeyTypeCorrect()
		}
	}

	// TODO(mattmoor): create KafkaBinding for the receive adapter.

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		var event *pkgreconciler.ReconcilerEvent
		isReconcilerEvent := pkgreconciler.EventAs(err, &event)
		if isReconcilerEvent && event.EventType != corev1.EventTypeNormal {
			logging.FromContext(ctx).Error("Unable to create the receive adapter. Reconciler error", zap.Error(err))
			return err
		} else if !isReconcilerEvent {
			logging.FromContext(ctx).Error("Unable to create the receive adapter. Generic error", zap.Error(err))
			return err
		}
	}
	src.Status.MarkDeployed(ra)
	src.Status.CloudEventAttributes = r.createCloudEventAttributes(src)

	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1beta1.KafkaSource, sinkURI *apis.URL) (*appsv1.Deployment, error) {
	raArgs := resources.ReceiveAdapterArgs{
		Image:          r.receiveAdapterImage,
		Source:         src,
		Labels:         resources.GetLabels(src.Name),
		SinkURI:        sinkURI.String(),
		AdditionalEnvs: r.configs.ToEnvVars(),
	}
	expected := resources.MakeReceiveAdapter(&raArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(ctx, expected.Name, metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(ctx, expected, metav1.CreateOptions{})
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
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ctx, ra, metav1.UpdateOptions{}); err != nil {
			return ra, err
		}
		return ra, deploymentUpdated(ra.Namespace, ra.Name)
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

//deleteReceiveAdapter deletes the receiver adapter deployment if any
func (r *Reconciler) deleteReceiveAdapter(ctx context.Context, src *v1beta1.KafkaSource) error {
	name := utils.GenerateFixedName(src, fmt.Sprintf("kafkasource-%s", src.Name))

	return r.KubeClientSet.AppsV1().Deployments(src.Namespace).Delete(ctx, name, metav1.DeleteOptions{})
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

func (r *Reconciler) createCloudEventAttributes(src *v1beta1.KafkaSource) []duckv1.CloudEventAttributes {
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(src.Spec.Topics))
	for i := range src.Spec.Topics {
		topics := strings.Split(src.Spec.Topics[i], ",")
		for _, topic := range topics {
			ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
				Type:   v1beta1.KafkaEventType,
				Source: v1beta1.KafkaEventSource(src.Namespace, src.Name, topic),
			})
		}
	}
	return ceAttributes
}
