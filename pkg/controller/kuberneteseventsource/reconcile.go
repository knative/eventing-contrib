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

package kuberneteseventsource

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/kuberneteseventsource/resources"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconciler reconciles a KubernetesEventSource object
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	dynamicClient       dynamic.Interface
	recorder            record.EventRecorder
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
	logger := logging.FromContext(ctx).Named(controllerAgentName)
	ctx = logging.WithLogger(ctx, logger)

	source, ok := object.(*sourcesv1alpha1.KubernetesEventSource)
	if !ok {
		logger.Errorf("could not find github source %v\n", object)
		return object, nil
	}

	logger.Debug("Reconciling", zap.String("name", source.Name), zap.String("namespace", source.Namespace))

	// See if the source has been deleted
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", zap.Error(err))
		return object, err
	}
	deleted := accessor.GetDeletionTimestamp() != nil

	source.Status.InitializeConditions()

	//TODO How do we ensure this never creates two container sources due to cache latency?
	// Is this possible?
	// 1. User creates
	// 2. KES gets into cache
	// 3. Reconcile: create ContainerSource A, update KES status
	// 4. KES gets into cache
	// 5. Reconcile: create ContainerSource B, update KES status
	// 6. KES gets into cache
	// 7. ContainerSource A gets into cache
	// 8. ContainerSource B gets into cache
	// 9. Reconcile: which ContainerSource?
	cs, err := r.getOwnedContainerSource(ctx, source)
	if err != nil {
		if errors.IsNotFound(err) && !deleted {
			cs := resources.MakeContainerSource(source, r.receiveAdapterImage)
			if err := controllerutil.SetControllerReference(source, cs, r.scheme); err != nil {
				return source, err
			}
			if err := r.client.Create(ctx, cs); err != nil {
				return source, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ContainerSourceCreated", "Created ContainerSource %q", cs.Name)
			// Wait for the ContainerSource to get a status
			return source, nil
		}
	}
	if deleted {
		return source, nil
	}
	// Update ContainerSource spec if it's changed
	expected := resources.MakeContainerSource(source, r.receiveAdapterImage)
	if !equality.Semantic.DeepEqual(cs.Spec, expected.Spec) {
		cs.Spec = expected.Spec
		if r.client.Update(ctx, cs); err != nil {
			return source, err
		}
	}

	// Copy ContainerSource conditions to source
	source.Status.Conditions = cs.Status.Conditions.DeepCopy()

	return source, nil
}

func (r *reconciler) getOwnedContainerSource(ctx context.Context, source *sourcesv1alpha1.KubernetesEventSource) (*sourcesv1alpha1.ContainerSource, error) {
	list := &sourcesv1alpha1.ContainerSourceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ContainerSource",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, cs := range list.Items {
		if metav1.IsControlledBy(&cs, source) {
			//TODO if there are >1 controlled, delete all but first?
			return &cs, nil
		}
	}
	return nil, errors.NewNotFound(sourcesv1alpha1.Resource("containersources"), "")
}
