/*
Copyright 2019 The Knative Authors.

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

package cloudeventsingresssource

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/eventing-sources/pkg/reconciler/cloudeventsingresssource/resources"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "cloudeventsingresssource-controller"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "CLOUDINGRESS_RA_IMAGE"
)

// Add creates a new CloudEventsIngressSource Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager) error {
	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	log.Println("Adding the CloudEventsIngressSource controller.")
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.CloudEventsIngressSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: raImage,
		},
	}

	return p.Add(mgr)
}

// reconciler reconciles a CloudEventsIngressSource object
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	receiveAdapterImage string
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	src, ok := object.(*v1alpha1.CloudEventsIngressSource)
	if !ok {
		logger.Error("could not find CloudEventsIngressSource source", zap.Any("object", object))
		return nil
	}

	// This Source attempts to reconcile two things.
	// 1. Determine the sink's URI.
	// 2. Create a knative service
	src.Status.InitializeConditions()

	sinkURI, err := sinks.GetSinkURI(ctx, r.client, src.Spec.Sink, src.Namespace)
	if err != nil {
		src.Status.MarkNoSink("NotFound", "")
		return err
	}
	src.Status.MarkSink(sinkURI)

	_, err = r.getOwnedService(ctx, src)
	if err != nil {
		if apierrors.IsNotFound(err) {
			err = r.createKnativeService(ctx, src, sinkURI)
			if err != nil {
				logger.Error("Unable to create the receive adapter", zap.Error(err))
				return err
			}
			src.Status.MarkDeployed()
			return nil
		}
		return err
	}

	return nil
}

func (r *reconciler) createKnativeService(ctx context.Context, src *v1alpha1.CloudEventsIngressSource, sinkURI string) error {
	args := resources.CloudEventsIngressSourceArgs{
		Image:   r.receiveAdapterImage,
		Source:  src,
		SinkURI: sinkURI,
	}
	expected := resources.MakeCloudEventsIngressSource(&args)
	logging.FromContext(ctx).Info("CloudEvent Object Created %s", expected)

	if err := controllerutil.SetControllerReference(src, expected, r.scheme); err != nil {
		return err
	}
	if err := r.client.Create(ctx, expected); err != nil {
		return err
	}
	logging.FromContext(ctx).Info("CloudEventsIngressSource created. %s", expected)
	return nil
}

func (r *reconciler) getOwnedService(ctx context.Context, source *v1alpha1.CloudEventsIngressSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: servingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}
