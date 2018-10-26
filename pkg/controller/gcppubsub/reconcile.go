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
	"cloud.google.com/go/pubsub"
	"context"
	errors2 "errors"
	"fmt"
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	finalizerName = controllerAgentName
)

type reconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface

	receiveAdapaterImage string
	serviceAccountName   string
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
	// 2. Create a receive adapter in the form of a Knative Service.
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

	sinkURI, err :=(&sdk.SinkUri{}).Get(ctx, src.Namespace, src.Spec.Sink, r.dynamicClient)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return src, err
	}
	src.Status.MarkSink(sinkURI)

	ra, err := r.createReceiveAdapter(ctx, src, sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return nil, err
	}
	raDomain := ra.Status.Domain
	if raDomain == "" {
		// The Receive Adapter does not yet have a domain set. Give it time to be reconciled.
		return src, errors2.New("receive adapter domain not set")
	}
	src.Status.MarkDeployed()

	err = r.createSubscription(ctx, src, raDomain)
	if err != nil {
		logger.Error("Unable to create the subscription", zap.Error(err), zap.String("receiveAdapterDomain", raDomain))
		return nil, err
	}
	src.Status.MarkSubscribed()

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

func (r *reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource, sinkURI string) (*servingv1alpha1.Service, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !errors.IsNotFound(err) {
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

func (r *reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource) (*servingv1alpha1.Service, error) {
	sl := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace: src.Namespace,
		LabelSelector: r.getLabelSelector(src),
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: servingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Service",
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
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
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
		"knative-eventing-source": controllerAgentName,
		"knative-eventing-source-name": src.Name,

	}
}

func (r *reconciler) makeReceiveAdapter(src *v1alpha1.GcpPubSubSource, sinkURI string) *servingv1alpha1.Service {
	return &servingv1alpha1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: src.Namespace,
			GenerateName: fmt.Sprintf("gcppubsub-%s", src.Name),
			Labels: getLabels(src),
		},
		Spec: servingv1alpha1.ServiceSpec{
			RunLatest:&servingv1alpha1.RunLatestType{
				Configuration:servingv1alpha1.ConfigurationSpec{
					RevisionTemplate:servingv1alpha1.RevisionTemplateSpec{
						Spec:servingv1alpha1.RevisionSpec{
							ServiceAccountName: r.serviceAccountName,
							Container: corev1.Container{
								Image: r.receiveAdapaterImage,
								Env: []corev1.EnvVar{
									{
										Name: "SINK_URI",
										Value: sinkURI,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *reconciler) createSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource, raDomain string) error {
	client, err := pubsub.NewClient(ctx, src.Spec.GoogleCloudProject) // TODO something about authing to GCP.
	if err != nil {
		return err
	}
	raURI := url.URL{
		Scheme: "http",
		Host: raDomain,
		Path: "/",
	}
	subID := client.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject).String()
	_, err = client.CreateSubscription(ctx, subID, pubsub.SubscriptionConfig{
		Topic: client.Topic(src.Spec.Topic),
		PushConfig: pubsub.PushConfig{
			Endpoint: raURI.String(),
		},
	})
	return err
}

func (r *reconciler) deleteSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource) error {
	client, err := pubsub.NewClient(ctx, src.Spec.GoogleCloudProject) // TODO something about authing to GCP.
	if err != nil {
		return err
	}
	sub := client.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
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
