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

package containersource

import (
	"context"
	"fmt"
	"github.com/knative/pkg/apis/duck"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"strings"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/containersource/resources"
	duckapis "github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type reconciler struct {
	client        client.Client
	scheme        *runtime.Scheme
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

const tryTargetable = true

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx)

	source, ok := object.(*v1alpha1.ContainerSource)
	if !ok {
		logger.Errorf("could not find container source %v\n", object)
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

	args := &resources.ContainerArguments{
		Name:      source.Name,
		Namespace: source.Namespace,
		Image:     source.Spec.Image,
		Args:      source.Spec.Args,
	}

	if uri, ok := sinkArg(source); ok {
		args.SinkInArgs = true
		source.Status.MarkSink(uri)
	} else {
		uri, err := r.getSinkUri(ctx, source)
		if err != nil {
			source.Status.MarkNoSink("NotFound", "")
			return source, err
		}
		source.Status.MarkSink(uri)
		args.Sink = uri
	}

	deploy, err := r.getDeployment(ctx, source)
	if err != nil {
		if errors.IsNotFound(err) {
			deploy, err = r.createDeployment(ctx, source, nil, args)
			if err != nil {
				r.recorder.Eventf(source, corev1.EventTypeNormal, "DeploymentBlocked", "waiting for %v", err)
				return object, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Deployed", "Created deployment %q", deploy.Name)
			source.Status.MarkDeploying("Deploying", "Created deployment %s", args.Name)
		} else {
			if deploy.Status.ReadyReplicas > 0 {
				source.Status.MarkDeployed()
			}
		}
	}

	return source, nil
}

func sinkArg(source *v1alpha1.ContainerSource) (string, bool) {
	for _, a := range source.Spec.Args {
		if strings.HasPrefix(a, "--sink=") {
			return strings.Replace(a, "--sink=", "", -1), true
		}
	}
	return "", false
}

func (r *reconciler) getSinkUri(ctx context.Context, source *v1alpha1.ContainerSource) (string, error) {
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

func (r *reconciler) getDeployment(ctx context.Context, source *v1alpha1.ContainerSource) (*appsv1.Deployment, error) {
	logger := logging.FromContext(ctx)

	list := &appsv1.DeploymentList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     source.Namespace,
			LabelSelector: labels.Everything(),
			// TODO this is here because the fake client needs it.
			// Remove this when it's no longer needed.
			Raw: &metav1.ListOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
				},
			},
		},
		list)
	if err != nil {
		logger.Errorf("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) createDeployment(ctx context.Context, source *v1alpha1.ContainerSource, org *appsv1.Deployment, args *resources.ContainerArguments) (*appsv1.Deployment, error) {
	deployment, err := resources.MakeDeployment(org, args)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(source, deployment, r.scheme); err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, deployment); err != nil {
		return nil, err
	}
	return deployment, nil
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
