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

package cronjobsource

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/knative/eventing-sources/pkg/controller/cronjobsource/resources"
	"github.com/knative/eventing-sources/pkg/controller/sinks"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client              client.Client
	dynamicClient       dynamic.Interface
	scheme              *runtime.Scheme
	receiveAdapterImage string
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

func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx).Desugar()

	src, ok := object.(*v1alpha1.CronJobSource)
	if !ok {
		logger.Error("could not find Cron Job source", zap.Any("object", object))
		return object, nil
	}

	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.

	src.Status.InitializeConditions()

	sinkURI, err := sinks.GetSinkURI(r.dynamicClient, src.Spec.Sink, src.Namespace)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return src, err
	}
	src.Status.MarkSink(sinkURI)

	_, err = r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return src, err
	}
	src.Status.MarkDeployed()

	return src, nil
}

func (r *reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource, sinkURI string) (*v1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	adapterArgs := resources.ReceiveAdapterArgs{
		Image:   r.receiveAdapterImage,
		Source:  src,
		Labels:  getLabels(src),
		SinkURI: sinkURI,
	}
	expected := resources.MakeReceiveAdapter(&adapterArgs)
	if ra != nil {
		if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
			ra.Spec.Template.Spec = expected.Spec.Template.Spec
			if err = r.client.Update(ctx, ra); err != nil {
				return ra, err
			}
			logging.FromContext(ctx).Desugar().Info("Receive Adapter updated.", zap.Any("receiveAdapter", ra))
		} else {
			logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		}
		return ra, nil
	}

	if err := controllerutil.SetControllerReference(src, expected, r.scheme); err != nil {
		return nil, err
	}
	if err = r.client.Create(ctx, expected); err != nil {
		return nil, err
	}
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Any("receiveAdapter", expected))
	return expected, err
}

func (r *reconciler) podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	if oldPodSpec.ServiceAccountName != newPodSpec.ServiceAccountName {
		return true
	}
	if oldPodSpec.Containers[0].Image != newPodSpec.Containers[0].Image {
		return true
	}
	if len(oldPodSpec.Containers[0].Env) != len(newPodSpec.Containers[0].Env) {
		return true
	}
	oldEnvMap := make(map[string]string)
	for _, e := range oldPodSpec.Containers[0].Env {
		oldEnvMap[e.Name] = e.Value
	}
	for _, e := range newPodSpec.Containers[0].Env {
		if oldValue, ok := oldEnvMap[e.Name]; !ok {
			return true
		} else if e.Value != oldValue {
			return true
		}
	}
	return false
}

func (r *reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.CronJobSource) (*v1.Deployment, error) {
	dl := &v1.DeploymentList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     src.Namespace,
		LabelSelector: r.getLabelSelector(src),
		// TODO this is only needed by the fake client. Real K8s does not need it. Remove it once
		// the fake is fixed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
		},
	},
		dl)

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list cronjobs: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) getLabelSelector(src *v1alpha1.CronJobSource) labels.Selector {
	return labels.SelectorFromSet(getLabels(src))
}

func getLabels(src *v1alpha1.CronJobSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      controllerAgentName,
		"knative-eventing-source-name": src.Name,
	}
}
