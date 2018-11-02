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

package githubsource

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/githubsource/resources"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	serving "github.com/knative/serving/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconciler reconciles a GitHubSource object
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	dynamicClient       dynamic.Interface
	recorder            record.EventRecorder
	servingClient       serving.Clientset
	receiveAdapterImage string
}

// TODO(n3wscott): To show the source is working, and while knative eventing is
// defining the ducktypes, tryTargetable allows us to point to a Service.service
// to validate the source is working.
const tryTargetable = true

// Reconcile reads that state of the cluster for a GitHubSource
// object and makes changes based on the state read and what is in the
// GitHubSource.Spec
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx)

	source, ok := object.(*v1alpha1.GitHubSource)
	if !ok {
		logger.Errorf("could not find github source %v\n", object)
		return object, nil
	}

	// See if the source has been deleted
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", zap.Error(err))
		return object, err
	}
	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		return object, nil
	}

	reconcileErr := r.reconcile(ctx, source)
	return source, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, source *v1alpha1.GitHubSource) error {
	source.Status.InitializeConditions()

	if source.Spec.Repository == "" {
		source.Status.MarkNotValid("RepositoryMissing", "")
		return errors.New("repository is empty")
	}
	if source.Spec.AccessToken.SecretKeyRef == nil {
		source.Status.MarkNotValid("AccessTokenMissing", "")
		return errors.New("access token ref is nil")
	}
	if source.Spec.SecretToken.SecretKeyRef == nil {
		source.Status.MarkNotValid("SecretTokenMissing", "")
		return errors.New("secret token ref is nil")
	}
	source.Status.MarkValid()

	uri, err := r.getSinkURI(ctx, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedService(ctx, source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc := resources.MakeService(source, r.receiveAdapterImage)
			if err := controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
				return err
			}
			if err := r.client.Create(ctx, ksvc); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
			// TODO: Mark Deploying
			// Wait for the Service to get a status
			return nil
		}
	} else {
		routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
		if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue {
			// TODO: Mark Deployed
		}
	}

	return nil
}

func (r *reconciler) getSinkURI(ctx context.Context, source *v1alpha1.GitHubSource) (string, error) {
	logger := logging.FromContext(ctx)

	// check to see if the source has provided a sink ref in the spec. Lets look for it.

	if source.Spec.Sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	obj, err := r.fetchObjectReference(ctx, source.Namespace, source.Spec.Sink)
	if err != nil {
		logger.Warnf("Failed to fetch sink target %+v: %s", source.Spec.Sink, zap.Error(err))
		return "", err
	}
	t := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(obj, &t)
	if err != nil {
		logger.Warnf("Failed to deserialize sink: %s", zap.Error(err))
		return "", err
	}

	if t.Status.Sinkable != nil {
		return fmt.Sprintf("http://%s/", t.Status.Sinkable.DomainInternal), nil
	}

	// for now we will try again as a targetable.
	if tryTargetable {
		t := duckv1alpha1.Target{}
		err = duck.FromUnstructured(obj, &t)
		if err != nil {
			logger.Warnf("Failed to deserialize targetable: %s", zap.Error(err))
			return "", err
		}

		if t.Status.Targetable != nil {
			return fmt.Sprintf("http://%s/", t.Status.Targetable.DomainInternal), nil
		}
	}

	return "", fmt.Errorf("sink does not contain sinkable")
}

func (r *reconciler) getOwnedService(ctx context.Context, source *v1alpha1.GitHubSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
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
			//TODO if there are >1 controlled, delete all but first?
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

// fetchObjectReference fetches an object based on ObjectReference.
func (r *reconciler) fetchObjectReference(ctx context.Context, namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	logger := logging.FromContext(ctx)

	resourceClient, err := r.CreateResourceInterface(namespace, ref)
	if err != nil {
		logger.Warnf("failed to create dynamic client resource: %v", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func (r *reconciler) CreateResourceInterface(namespace string, ref *corev1.ObjectReference) (dynamic.ResourceInterface, error) {
	rc := r.dynamicClient.Resource(duckapis.KindToResource(ref.GroupVersionKind()))
	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil

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
