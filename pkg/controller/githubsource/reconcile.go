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
	"fmt"

	"go.uber.org/zap"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconciler reconciles a GitHubSource object
type reconciler struct {
	client        client.Client
	scheme        *runtime.Scheme
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
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

	source.Status.InitializeConditions()

	uri, err := r.getSinkURI(ctx, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return source, err
	}
	source.Status.MarkSink(uri)

	return source, nil
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
