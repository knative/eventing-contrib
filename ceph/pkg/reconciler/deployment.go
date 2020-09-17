package reconciler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// newDeploymentCreated makes a new reconciler event with event type Normal, and
// reason DeploymentCreated.
func newDeploymentCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "DeploymentCreated", "created deployment: \"%s/%s\"", namespace, name)
}

// newDeploymentFailed makes a new reconciler event with event type Warning, and
// reason DeploymentFailed.
func newDeploymentFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DeploymentFailed", "failed to create deployment: \"%s/%s\", %w", namespace, name, err)
}

// newDeploymentUpdated makes a new reconciler event with event type Normal, and
// reason DeploymentUpdated.
func newDeploymentUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "DeploymentUpdated", "updated deployment: \"%s/%s\"", namespace, name)
}

type DeploymentReconciler struct {
	KubeClientSet kubernetes.Interface
}

func (r *DeploymentReconciler) ReconcileDeployment(ctx context.Context, owner kmeta.OwnerRefable, expected *appsv1.Deployment) (*appsv1.Deployment, pkgreconciler.Event) {
	namespace := owner.GetObjectMeta().GetNamespace()
	ra, err := r.KubeClientSet.AppsV1().Deployments(namespace).Get(ctx, expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.KubeClientSet.AppsV1().Deployments(namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, newDeploymentFailed(expected.Namespace, expected.Name, err)
		}
		return ra, newDeploymentCreated(ra.Namespace, ra.Name)
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter %q: %v", expected.Name, err)
	} else if !metav1.IsControlledBy(ra, owner.GetObjectMeta()) {
		return nil, fmt.Errorf("deployment %q is not owned by %s %q",
			ra.Name, owner.GetGroupVersionKind().Kind, owner.GetObjectMeta().GetName())
	} else if r.podSpecImageSync(expected.Spec.Template.Spec, ra.Spec.Template.Spec) {
		if ra, err = r.KubeClientSet.AppsV1().Deployments(namespace).Update(ctx, ra, metav1.UpdateOptions{}); err != nil {
			return ra, err
		}
		return ra, newDeploymentUpdated(ra.Namespace, ra.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func (r *DeploymentReconciler) FindOwned(ctx context.Context, owner kmeta.OwnerRefable, selector labels.Selector) (*appsv1.Deployment, error) {
	dl, err := r.KubeClientSet.AppsV1().Deployments(owner.GetObjectMeta().GetNamespace()).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		logging.FromContext(ctx).Error("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, owner.GetObjectMeta()) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func getContainer(name string, spec corev1.PodSpec) (int, *corev1.Container) {
	for i, c := range spec.Containers {
		if c.Name == name {
			return i, &c
		}
	}
	return -1, nil
}

// Returns false if an update is needed.
func (r *DeploymentReconciler) podSpecImageSync(expected corev1.PodSpec, now corev1.PodSpec) bool {
	// got needs all of the containers that want as, but it is allowed to have more.
	dirty := false
	for _, ec := range expected.Containers {
		n, nc := getContainer(ec.Name, now)
		if nc == nil {
			now.Containers = append(now.Containers, ec)
			dirty = true
			continue
		}
		if nc.Image != ec.Image {
			now.Containers[n].Image = ec.Image
			dirty = true
		}
	}
	return dirty
}
