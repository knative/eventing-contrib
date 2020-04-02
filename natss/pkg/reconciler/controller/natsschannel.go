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

package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing/pkg/reconciler/names"

	"knative.dev/eventing-contrib/natss/pkg/apis/messaging/v1alpha1"
	natssChannelReconciler "knative.dev/eventing-contrib/natss/pkg/client/injection/reconciler/messaging/v1alpha1/natsschannel"
	listers "knative.dev/eventing-contrib/natss/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing-contrib/natss/pkg/reconciler/controller/resources"
)

const (
	ReconcilerName = "NatssChannel"

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled            = "NatssChannelReconciled"
	channelReconcileFailed       = "NatssChannelReconcileFailed"
	dispatcherDeploymentNotFound = "DispatcherDeploymentDoesNotExist"
	dispatcherDeploymentFailed   = "DispatcherDeploymentFailed"
	dispatcherServiceFailed      = "DispatcherServiceFailed"
	dispatcherServiceNotFound    = "DispatcherServiceDoesNotExist"
	dispatcherEndpointsNotFound  = "DispatcherEndpointsDoesNotExist"
	dispatcherEndpointsFailed    = "DispatcherEndpointsFailed"
	channelServiceFailed         = "ChannelServiceFailed"

	dispatcherName = "natss-ch-dispatcher"

	// reconciled normal message format (namespace, name)
	reconciledNormalFmt = ReconcilerName + " reconciled: \"%s/%s\""
)

// Reconciler reconciles NATSS Channels.
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string

	natsschannelLister   listers.NatssChannelLister
	natsschannelInformer cache.SharedIndexInformer
	deploymentLister     appsv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
	endpointsLister      corev1listers.EndpointsLister
}

var _ natssChannelReconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, nc *v1alpha1.NatssChannel) reconciler.Event {
	nc.Status.InitializeConditions()

	logger := logging.FromContext(ctx)
	// Verify channel is valid.
	nc.SetDefaults(ctx)
	if err := nc.Validate(ctx); err != nil {
		logger.Error("Invalid natss channel", zap.String("channel", nc.Name), zap.Error(err))
		return err
	}

	// We reconcile the status of the Channel by looking at:
	// 1. Dispatcher Deployment for it's readiness.
	// 2. Dispatcher k8s Service for it's existence.
	// 3. Dispatcher endpoints to ensure that there's something backing the Service.
	// 4. K8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service.

	// Get the Dispatcher Deployment and propagate the status to the Channel
	d, err := r.deploymentLister.Deployments(r.dispatcherNamespace).Get(r.dispatcherDeploymentName)
	if err != nil {
		logger.Error("Unable to get the dispatcher Deployment", zap.Error(err))
		if apierrs.IsNotFound(err) {
			nc.Status.MarkDispatcherFailed(dispatcherDeploymentNotFound, "Dispatcher Deployment does not exist")
		} else {
			nc.Status.MarkDispatcherFailed(dispatcherDeploymentFailed, "Failed to get dispatcher Deployment")
		}
		return newError(err)
	}
	nc.Status.PropagateDispatcherStatus(&d.Status)

	// Get the Dispatcher Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.serviceLister.Services(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		logger.Error("Unable to get the dispatcher service", zap.Error(err))
		if apierrs.IsNotFound(err) {
			nc.Status.MarkServiceFailed(dispatcherServiceNotFound, "Dispatcher Service does not exist")
		} else {
			nc.Status.MarkServiceFailed(dispatcherServiceFailed, "Failed to get dispatcher service")
		}
		return newError(err)
	}
	nc.Status.MarkServiceTrue()

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		logger.Error("Unable to get the dispatcher endpoints", zap.Error(err))
		if apierrs.IsNotFound(err) {
			nc.Status.MarkEndpointsFailed(dispatcherEndpointsNotFound, "Dispatcher Endpoints does not exist")
		} else {
			nc.Status.MarkEndpointsFailed(dispatcherEndpointsFailed, "Failed to get dispatcher endpoints")
		}
		return newError(err)
	}

	if len(e.Subsets) == 0 {
		logger.Error("No endpoints found for Dispatcher service", zap.Error(err))
		nc.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		return fmt.Errorf("there are no endpoints ready for Dispatcher service %s", r.dispatcherServiceName)
	}
	nc.Status.MarkEndpointsTrue()

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via ExternalName
	svc, err := r.reconcileChannelService(ctx, nc)
	if err != nil {
		nc.Status.MarkChannelServiceFailed(channelServiceFailed, fmt.Sprintf("Channel Service failed: %s", err))
		return newError(err)
	}
	nc.Status.MarkChannelServiceTrue()
	nc.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return newReconciledNormal(nc.Namespace, nc.Name)
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *v1alpha1.NatssChannel) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(channel.Namespace).Get(resources.MakeChannelServiceName(channel.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(channel, resources.ExternalService(r.dispatcherNamespace, r.dispatcherServiceName))
			if err != nil {
				logger.Error("Failed to create the channel service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.kubeClientSet.CoreV1().Services(channel.Namespace).Create(svc)
			if err != nil {
				logger.Error("Failed to create the channel service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		}
		logger.Error("Unable to get the channel service", zap.Error(err))
		return nil, err
	}
	// Check to make sure that the NatssChannel owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, channel) {
		return nil, fmt.Errorf("natsschannel: %s/%s does not own Service: %q", channel.Namespace, channel.Name, svc.Name)
	}
	return svc, nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, nc *v1alpha1.NatssChannel) reconciler.Event {
	return newReconciledNormal(nc.Namespace, nc.Name)
}

func newReconciledNormal(namespace, name string) reconciler.Event {
	return reconciler.NewEvent(corev1.EventTypeNormal, channelReconciled, reconciledNormalFmt, namespace, name)
}

func newError(err error) error {
	return fmt.Errorf(ReconcilerName+" reconciliation failed with: %s", err)
}
