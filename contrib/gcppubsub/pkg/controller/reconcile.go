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

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/knative/eventing-sources/contrib/gcppubsub/pkg/controller/resources"

	"github.com/knative/eventing-sources/pkg/controller/sinks"

	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing-sources/contrib/gcppubsub/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	finalizerName = controllerAgentName
)

// gcpPubSubClientCreator creates a real GCP PubSub client. It should always be used, except during
// unit tests.
func gcpPubSubClientCreator(ctx context.Context, googleCloudProject string) (pubSubClient, error) {
	// Auth to GCP is handled by having the GOOGLE_APPLICATION_CREDENTIALS environment variable
	// pointing at a credential file.
	psc, err := pubsub.NewClient(ctx, googleCloudProject)
	if err != nil {
		return nil, err
	}
	return &realGcpPubSubClient{
		client: psc,
	}, nil
}

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme

	pubSubClientCreator pubSubClientCreator

	receiveAdapterImage string
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
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
	// 3. Register that receive adapter as a Pull endpoint for the specified GCP PubSub Topic.
	//     - This needs to deregister during deletion.
	// Because there is something that must happen during deletion, we add this controller as a
	// finalizer to every GcpPubSubSource.

	// See if the source has been deleted.
	deletionTimestamp := src.DeletionTimestamp
	if deletionTimestamp != nil {
		err := r.deleteSubscription(ctx, src)
		if err != nil {
			logger.Error("Unable to delete the Subscription", zap.Error(err))
			return src, err
		}
		r.removeFinalizer(src)
		return src, nil
	}

	r.addFinalizer(src)

	src.Status.InitializeConditions()

	sinkURI, err := sinks.GetSinkURI(ctx, r.client, src.Spec.Sink, src.Namespace)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return src, err
	}
	src.Status.MarkSink(sinkURI)

	sub, err := r.createSubscription(ctx, src)
	if err != nil {
		logger.Error("Unable to create the subscription", zap.Error(err))
		return src, err
	}
	src.Status.MarkSubscribed()

	_, err = r.createReceiveAdapter(ctx, src, sub.ID(), sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return src, err
	}
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

func (r *reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource, subscriptionID, sinkURI string) (*v1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		return ra, nil
	}
	svc := resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:          r.receiveAdapterImage,
		Source:         src,
		Labels:         getLabels(src),
		SubscriptionID: subscriptionID,
		SinkURI:        sinkURI,
	})
	if err := controllerutil.SetControllerReference(src, svc, r.scheme); err != nil {
		return nil, err
	}
	err = r.client.Create(ctx, svc)
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", svc))
	return svc, err
}

func (r *reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.GcpPubSubSource) (*v1.Deployment, error) {
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
		logging.FromContext(ctx).Desugar().Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) getLabelSelector(src *v1alpha1.GcpPubSubSource) labels.Selector {
	return labels.SelectorFromSet(getLabels(src))
}

func getLabels(src *v1alpha1.GcpPubSubSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      controllerAgentName,
		"knative-eventing-source-name": src.Name,
	}
}

func (r *reconciler) createSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource) (pubSubSubscription, error) {
	psc, err := r.pubSubClientCreator(ctx, src.Spec.GoogleCloudProject)
	if err != nil {
		return nil, err
	}
	sub := psc.SubscriptionInProject(generateSubName(src), src.Spec.GoogleCloudProject)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, err
	} else if exists {
		logging.FromContext(ctx).Info("Reusing existing subscription.")
		return sub, nil
	}
	createdSub, err := psc.CreateSubscription(ctx, sub.ID(), pubsub.SubscriptionConfig{
		Topic: psc.Topic(src.Spec.Topic),
	})
	if err != nil {
		logging.FromContext(ctx).Desugar().Info("Error creating new subscription", zap.Error(err))
	} else {
		logging.FromContext(ctx).Desugar().Info("Created new subscription", zap.Any("subscription", createdSub))
	}
	return createdSub, err
}

func (r *reconciler) deleteSubscription(ctx context.Context, src *v1alpha1.GcpPubSubSource) error {
	psc, err := r.pubSubClientCreator(ctx, src.Spec.GoogleCloudProject)
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
