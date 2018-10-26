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

package gcppubsub

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	finalizerName = controllerAgentName
)

var (
	trueVal = true
)

type reconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface

	receiveAdapterImage              string
	receiveAdapterServiceAccountName string
	receiveAdapterCredsSecret        string
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

	src, ok := object.(*v1alpha1.GcpPubSubSource)
	if !ok {
		logger.Error("could not find GcpPubSub source", zap.Any("object", object))
		return object, nil
	}

	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this GcpPubSubSource is deleted.
	// 3. Register that receive adapter as a Push endpoint for the specified GCP PubSub Topic.
	//     - This need to deregistered during deletion.
	// Because there is something that must happen during deletion, we add this controller as a
	// finalizer to every GcpPubSubSource.

	// See if the source has been deleted.
	deletionTimestamp := src.DeletionTimestamp
	if deletionTimestamp != nil {
		err := r.deleteSubscription(ctx, src)
		if err != nil {
			logger.Error("Unable to delete the Subscription", zap.Error(err))
			return nil, err
		}
		r.removeFinalizer(src)
		return src, nil
	}

	r.addFinalizer(src)

	src.Status.InitializeConditions()

	sinkURI, err := sdk.GetSinkUri(ctx, r.dynamicClient, src.Spec.Sink, src.Namespace)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return src, err
	}
	src.Status.MarkSink(sinkURI)

	if err = r.createSubscription(ctx, src); err != nil {
		logger.Error("Unable to create the subscription", zap.Error(err))
		return nil, err
	}
	src.Status.MarkSubscribed()

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return nil, err
	}
	logger.Info("Receive Adapter created.", zap.Any("receiveAdapter", ra))
	src.Status.MarkDeployed()

	return src, nil
}

func (r *reconciler) addFinalizer(s *v1alpha1.GcpPubSubSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(s *v1alpha1.GcpPubSubSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource, sinkURI string) (*v1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		return ra, nil
	}
	svc := r.makeReceiveAdapter(src, sinkURI)
	err = r.client.Create(ctx, svc)
	return svc, err
}

func (r *reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource) (*v1.Deployment, error) {
	sl := &v1.DeploymentList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     src.Namespace,
		LabelSelector: r.getLabelSelector(src),
		// TODO this is only needed by the fake client. Real K8s does not need it. Remove it once
		// the fake is fixed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Deployment",
			},
		},
	},
		sl)

	if err != nil {
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, svc := range sl.Items {
		if metav1.IsControlledBy(&svc, src) {
			return &svc, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) getLabelSelector(src *v1alpha1.GcpPubSubSource) labels.Selector {
	ls := labels.NewSelector()
	for k, v := range getLabels(src) {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			// Probably fail.
		}
		ls.Add(*req)
	}
	return ls
}

func getLabels(src *v1alpha1.GcpPubSubSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      controllerAgentName,
		"knative-eventing-source-name": src.Name,
	}
}

func (r *reconciler) makeReceiveAdapter(src *v1alpha1.GcpPubSubSource, sinkURI string) *v1.Deployment {
	credsVolume := "google-cloud-key"
	credsMountPath := "/var/secrets/google"
	credsFile := fmt.Sprintf("%s/key.json", credsMountPath)
	var replicas int32 = 1
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    src.Namespace,
			GenerateName: fmt.Sprintf("gcppubsub-%s", src.Name),
			Labels:       getLabels(src),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "GcpPubSubSource",
					Name:       src.Name,
					UID:        src.UID,
					Controller: &trueVal,
				},
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(src),
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: r.receiveAdapterServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: r.receiveAdapterImage,
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: credsFile,
								},
								{
									Name:  "GCPPUBSUB_PROJECT",
									Value: src.Spec.GoogleCloudProject,
								},
								{
									Name:  "GCPPUBSUB_TOPIC",
									Value: src.Spec.Topic,
								},
								{
									Name:  "SINK_URI",
									Value: sinkURI,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      credsVolume,
									MountPath: credsMountPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: credsVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: r.receiveAdapterCredsSecret,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *reconciler) createSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource) error {
	psc, err := pubsub.NewClient(ctx, src.Spec.GoogleCloudProject) // TODO something about authing to GCP.
	if err != nil {
		return err
	}
	sub := psc.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
	if exists, err := sub.Exists(ctx); err != nil {
		return err
	} else if exists {
		return nil
	}
	_, err = psc.CreateSubscription(ctx, sub.ID(), pubsub.SubscriptionConfig{
		Topic: psc.Topic(src.Spec.Topic),
	})
	return err
}

func (r *reconciler) deleteSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource) error {
	psc, err := pubsub.NewClient(ctx, src.Spec.GoogleCloudProject) // TODO something about authing to GCP.
	if err != nil {
		return err
	}
	sub := psc.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
	if exists, err := sub.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}
	return sub.Delete(ctx)
}

func generateSubName(src *v1alpha1.GcpPubSubSource) string {
	return fmt.Sprintf("knative-eventing-%s-%s-%s", src.Namespace, src.Name, src.UID)
}
