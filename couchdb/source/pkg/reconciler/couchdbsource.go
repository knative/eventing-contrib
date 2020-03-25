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
	"net/url"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	cdbreconciler "knative.dev/eventing-contrib/couchdb/source/pkg/client/injection/reconciler/sources/v1alpha1/couchdbsource"
	"knative.dev/eventing-contrib/couchdb/source/pkg/reconciler/resources"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	couchdbsourceDeploymentCreated = "CouchDbSourceDeploymentCreated"
	couchdbsourceDeploymentUpdated = "CouchDbSourceDeploymentUpdated"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "COUCHDB_RA_IMAGE"
)

// Reconciler reconciles a CouchDbSource object
type Reconciler struct {
	*reconciler.Base

	receiveAdapterImage string

	// listers index properties about resources
	deploymentLister appsv1listers.DeploymentLister
	eventTypeLister  eventinglisters.EventTypeLister

	sinkResolver *resolver.URIResolver
}

var _ cdbreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.CouchDbSource) pkgreconciler.Event {
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
			dest.Ref.Namespace = source.GetNamespace()
		}
	}

	sinkURI, err := r.sinkResolver.URIFromDestinationV1(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return fmt.Errorf("getting sink URI: %v", err)
	}

	source.Status.MarkSink(sinkURI)

	ra, err := r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		r.Logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	// Update source status// Update source status
	source.Status.PropagateDeploymentAvailability(ra)

	ceSource, err := r.makeEventSource(source)
	if err != nil {
		r.Logger.Error("Unable to create the CloudEvents source", zap.Error(err))
		return err
	}

	source.Status.CloudEventAttributes = r.createCloudEventAttributes(ceSource)
	return nil
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.CouchDbSource, sinkURI *apis.URL) (*appsv1.Deployment, error) {
	eventSource, err := r.makeEventSource(src)
	if err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Debug("event source", zap.Any("source", eventSource))

	adapterArgs := resources.ReceiveAdapterArgs{
		EventSource: eventSource,
		Image:       r.receiveAdapterImage,
		Source:      src,
		Labels:      resources.Labels(src.Name),
		SinkURI:     sinkURI.String(),
	}
	expected := resources.MakeReceiveAdapter(&adapterArgs)

	ra, err := r.KubeClientSet.AppsV1().Deployments(src.Namespace).Get(expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
		r.Recorder.Eventf(src, corev1.EventTypeNormal, couchdbsourceDeploymentCreated, "Deployment created, error: %v", err)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by CouchDbSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.KubeClientSet.AppsV1().Deployments(src.Namespace).Update(ra); err != nil {
			return ra, err
		}
		r.Recorder.Eventf(src, corev1.EventTypeNormal, couchdbsourceDeploymentUpdated, "Deployment updated")
		return ra, nil
	} else {
		logging.FromContext(ctx).Debug("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
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

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.CouchDbSource) (*appsv1.Deployment, error) {
	dl, err := r.deploymentLister.Deployments(src.Namespace).List(r.getLabelSelector(src))
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl {
		if metav1.IsControlledBy(dep, src) {
			return dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getLabelSelector(src *v1alpha1.CouchDbSource) labels.Selector {
	return labels.SelectorFromSet(resources.Labels(src.Name))
}

// MakeEventSource computes the Cloud Event source attribute for the given source
func (r *Reconciler) makeEventSource(src *v1alpha1.CouchDbSource) (string, error) {
	namespace := src.Spec.CouchDbCredentials.Namespace
	if namespace == "" {
		namespace = src.Namespace
	}

	secret, err := r.KubeClientSet.CoreV1().Secrets(namespace).Get(src.Spec.CouchDbCredentials.Name, metav1.GetOptions{})
	if err != nil {
		r.Logger.Error("Unable to read CouchDB credentials secret", zap.Error(err))
		return "", err
	}
	rawurl, ok := secret.Data["url"]
	if !ok {
		r.Logger.Error("Unable to get CouchDB url field", zap.Any("secretName", secret.Name), zap.Any("secretNamespace", secret.Namespace))
		return "", err
	}

	url, err := url.Parse(string(rawurl))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", url.Hostname(), src.Spec.Database), nil
}

func (r *Reconciler) createCloudEventAttributes(ceSource string) []duckv1.CloudEventAttributes {
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(v1alpha1.CouchDbSourceEventTypes))
	for _, couchDbSourceEventType := range v1alpha1.CouchDbSourceEventTypes {
		ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
			Type:   couchDbSourceEventType,
			Source: ceSource,
		})
	}
	return ceAttributes
}
