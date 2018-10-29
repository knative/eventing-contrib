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

package edgesource

import (
	"context"
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reconciler struct {
	client        client.Client
	scheme        *runtime.Scheme
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx)

	source, ok := object.(*v1alpha1.EdgeSource)
	if !ok {
		logger.Errorf("could not find edge source %v\n", object)
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

	uri, err := sinks.GetSinkURI(r.dynamicClient, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return source, err
	}
	source.Status.MarkSink(uri)

	// TODO: reconcile

	return source, nil
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
