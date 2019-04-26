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

package apiserversource

import (
	"context"
	"fmt"
	"os"

	v1alpha1 "github.com/knative/eventing-sources/contrib/apiserver/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/contrib/apiserver/pkg/controller/apiserversource/resources"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	controllerAgentName = "apiserver-source-controller"

	// raImageEnvVar is the name of the environment variable that contains the receive adapter's
	// image. It must be defined.
	raImageEnvVar = "APISERVER_RA_IMAGE"
)

// Add creates a new ApiServerSource Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, l *zap.SugaredLogger) error {
	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable '%s' not defined", raImageEnvVar)
	}

	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.ApiServerSource{},
		Reconciler: &reconciler{
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: raImage,
		},
	}

	return p.Add(mgr, l)
}

// reconciler reconciles a ApiServerSource object
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

	source, ok := object.(*v1alpha1.ApiServerSource)
	if !ok {
		logger.Errorf("could not find apiserver source %v", object)
		return nil
	}

	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := source.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		// The deployment will be automatically GCed
		return nil
	}

	source.Status.InitializeConditions()

	sinkURI, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(sinkURI)

	_, err = r.createReceiveAdapter(ctx, source, sinkURI)
	if err != nil {
		logger.Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}

	// Update source status
	source.Status.MarkDeployed()

	return nil
}

func (r *reconciler) createReceiveAdapter(ctx context.Context, source *v1alpha1.ApiServerSource, sinkURI string) (*appsv1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, source)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}
	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		return ra, nil
	}
	svc := resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:   r.receiveAdapterImage,
		Source:  source,
		Labels:  getLabels(source),
		SinkURI: sinkURI,
	})
	// Make the source own the the adapter so that when the source is deleted, the adapter is GC.
	if err := controllerutil.SetControllerReference(source, svc, r.scheme); err != nil {
		return nil, err
	}
	err = r.client.Create(ctx, svc)
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", svc))
	return svc, err
}

func (r *reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.ApiServerSource) (*appsv1.Deployment, error) {
	dl := &appsv1.DeploymentList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     src.Namespace,
		LabelSelector: r.getLabelSelector(src),
		// TODO this is only needed by the fake client. Real K8s does not need it. Remove it once
		// the fake is fixed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
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

func (r *reconciler) getLabelSelector(src *v1alpha1.ApiServerSource) labels.Selector {
	return labels.SelectorFromSet(getLabels(src))
}

func getLabels(src *v1alpha1.ApiServerSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      controllerAgentName,
		"knative-eventing-source-name": src.Name,
	}
}
