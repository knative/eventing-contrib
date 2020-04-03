/*
Copyright 2020 The Knative Authors

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

package testing

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
)

// CamelSourceOption enables further configuration of a CamelSource.
type CamelSourceOption func(*v1alpha1.CamelSource)

// NewCamelSource creates an CamelSource with CamelSourceOptions.
func NewCamelSource(name, namespace string, generation int64, ncopt ...CamelSourceOption) *v1alpha1.CamelSource {
	nc := &v1alpha1.CamelSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: generation,
			UID:        "abc-123",
		},
		Spec: v1alpha1.CamelSourceSpec{},
	}
	for _, opt := range ncopt {
		opt(nc)
	}
	return nc
}

func WithInitCamelSource(nc *v1alpha1.CamelSource) {
	nc.Status.InitializeConditions()
	nc.Status.ObservedGeneration = nc.Generation
}

func WithCamelSourceDeleted(nc *v1alpha1.CamelSource) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	nc.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithCamelSourceSpec(spec v1alpha1.CamelSourceSpec) CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Spec = spec
	}
}

func WithCamelSourceSink(uri string) CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Status.MarkSink(uri)
	}
}

func WithCamelSourceSinkNotFound() CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Status.MarkNoSink("NotFound", "")
	}
}

func WithCamelSourceDeploying() CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Status.MarkDeploying("Deploying", "Created integration test-camel-source-*")
	}
}

func WithCamelSourceIntegrationUpdated() CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Status.MarkDeploying("IntegrationUpdated", "Updated integration test-camel-source-*")
	}
}

func WithCamelSourceDeployed() CamelSourceOption {
	return func(nc *v1alpha1.CamelSource) {
		nc.Status.MarkDeployed()
	}
}

// TODO:
//
//
//func WithCamelSourceConfigReady() CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkConfigTrue()
//	}
//}
//
//func WithCamelSourceDeploymentNotReady(reason, message string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkDispatcherFailed(reason, message)
//	}
//}
//
//func WithCamelSourceDeploymentReady() CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.PropagateDispatcherStatus(&appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}})
//	}
//}
//
//func WithCamelSourceServicetNotReady(reason, message string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkServiceFailed(reason, message)
//	}
//}
//
//func WithCamelSourceServiceReady() CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkServiceTrue()
//	}
//}
//
//func WithCamelSourceChannelServicetNotReady(reason, message string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkChannelServiceFailed(reason, message)
//	}
//}
//
//func WithCamelSourceChannelServiceReady() CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkChannelServiceTrue()
//	}
//}
//
//func WithCamelSourceEndpointsNotReady(reason, message string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkEndpointsFailed(reason, message)
//	}
//}
//
//func WithCamelSourceEndpointsReady() CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.MarkEndpointsTrue()
//	}
//}
//
//func WithCamelSourceAddress(a string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		nc.Status.SetAddress(&apis.URL{
//			Scheme: "http",
//			Host:   a,
//		})
//	}
//}
//
//func WithKafkaFinalizer(finalizerName string) CamelSourceOption {
//	return func(nc *v1alpha1.CamelSource) {
//		finalizers := sets.NewString(nc.Finalizers...)
//		finalizers.Insert(finalizerName)
//		nc.SetFinalizers(finalizers.List())
//	}
//}
