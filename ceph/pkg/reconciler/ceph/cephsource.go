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

package ceph

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	"knative.dev/eventing-contrib/ceph/pkg/apis/sources/v1alpha1"
	reconcilercephsource "knative.dev/eventing-contrib/ceph/pkg/client/injection/reconciler/sources/v1alpha1/cephsource"
	"knative.dev/eventing-contrib/ceph/pkg/reconciler"
	"knative.dev/eventing-contrib/ceph/pkg/reconciler/ceph/resources"
)

// Reconciler reconciles a CephSource object
type Reconciler struct {
	ReceiveAdapterImage string `envconfig:"CEPH_SOURCE_RA_IMAGE" required:"true"`

	dr  *reconciler.DeploymentReconciler
	sbr *reconciler.SinkBindingReconciler

	configAccessor reconcilersource.ConfigAccessor
}

// Check that our Reconciler implements Interface
var _ reconcilercephsource.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.CephSource) pkgreconciler.Event {
	ra, event := r.dr.ReconcileDeployment(ctx, src, resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:          r.ReceiveAdapterImage,
		Source:         src,
		Labels:         resources.Labels(src.Name),
		AdditionalEnvs: r.configAccessor.ToEnvVars(), // Grab config envs for tracing/logging/metrics
	}))
	if ra != nil {
		src.Status.PropagateDeploymentAvailability(ra)
	}
	if event != nil {
		logging.FromContext(ctx).Infof("returning because event from ReconcileDeployment")
		return event
	}

	if ra != nil {
		logging.FromContext(ctx).Info("going to ReconcileSinkBinding")
		sb, event := r.sbr.ReconcileSinkBinding(ctx, src, src.Spec.SourceSpec, tracker.Reference{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Namespace:  ra.Namespace,
			Name:       ra.Name,
		})
		logging.FromContext(ctx).Infof("ReconcileSinkBinding returned %#v", sb)
		if sb != nil {
			src.Status.MarkSink(sb.Status.SinkURI)
		}
		if event != nil {
			return event
		}
	}

	return nil
}
