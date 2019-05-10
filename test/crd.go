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

package test

// crd contains functions that construct boilerplate CRD definitions.

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	apiVersion = "eventing.knative.dev/v1alpha1"
)

// Route returns a Route object in namespace
func Route(name string, namespace string, configName string) *servingv1alpha1.Route {
	return &servingv1alpha1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: servingv1alpha1.RouteSpec{
			Traffic: []servingv1alpha1.TrafficTarget{
				{
					ConfigurationName: configName,
					Percent:           100,
				},
			},
		},
	}
}

// Configuration returns a Configuration object in namespace with name
// that uses the image specified by imagePath.
func Configuration(name string, namespace string, imagePath string) *servingv1alpha1.Configuration {
	return &servingv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: servingv1alpha1.ConfigurationSpec{
			RevisionTemplate: servingv1alpha1.RevisionTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"knative.dev/type": "container"},
				},
				Spec: servingv1alpha1.RevisionSpec{
					Container: corev1.Container{
						Image: imagePath,
					},
				},
			},
		},
	}
}

// ClusterChannelProvisioner returns a ClusterChannelProvisioner for a given name
func ClusterChannelProvisioner(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference("ClusterChannelProvisioner", apiVersion, name)
}

// ChannelRef returns an ObjectReference for a given Channel Name
func ChannelRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference("Channel", apiVersion, name)
}

// Channel returns a Channel with the specified provisioner
func Channel(name string, namespace string, provisioner *corev1.ObjectReference) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: provisioner,
		},
	}
}

// SubscriberSpecForRoute returns a SubscriberSpec for a given Knative Service.
func SubscriberSpecForRoute(name string) *v1alpha1.SubscriberSpec {
	return &v1alpha1.SubscriberSpec{
		Ref: &corev1.ObjectReference{
			Kind:       "Route",
			APIVersion: "serving.knative.dev/v1alpha1",
			Name:       name,
		},
	}
}

// Subscription returns a Subscription
func Subscription(name string, namespace string, channel *corev1.ObjectReference, subscriber *v1alpha1.SubscriberSpec, reply *v1alpha1.ReplyStrategy) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel:    *channel,
			Subscriber: subscriber,
			Reply:      reply,
		},
	}
}
