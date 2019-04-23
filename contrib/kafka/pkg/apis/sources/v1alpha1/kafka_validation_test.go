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

package v1alpha1

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

var (
	fullSpec = KafkaSourceSpec{
		BootstrapServers: "servers",
		Topics:           "topics",
		ConsumerGroup:    "group",
		Sink: &corev1.ObjectReference{
			APIVersion: "foo",
			Kind:       "bar",
			Namespace:  "baz",
			Name:       "qux",
		},
		ServiceAccountName: "service-account-name",
	}
)

func TestKafkaSourceCheckImmutableFields(t *testing.T) {
	testCases := map[string]struct {
		orig    *KafkaSourceSpec
		updated KafkaSourceSpec
		allowed bool
	}{
		"nil orig": {
			updated: fullSpec,
			allowed: true,
		},
		"Topic changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics:             "some-other-topic",
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Bootstrap servers changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				BootstrapServers:   "server1,server2",
				Sink:               fullSpec.Sink,
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.APIVersion changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics: fullSpec.Topics,
				Sink: &corev1.ObjectReference{
					APIVersion: "some-other-api-version",
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Kind changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics: fullSpec.Topics,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       "some-other-kind",
					Namespace:  fullSpec.Sink.Namespace,
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Namespace changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics: fullSpec.Topics,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  "some-other-namespace",
					Name:       fullSpec.Sink.Name,
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"Sink.Name changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics: fullSpec.Topics,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       "some-other-name",
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"ServiceAccountName changed": {
			orig: &fullSpec,
			updated: KafkaSourceSpec{
				Topics: fullSpec.Topics,
				Sink: &corev1.ObjectReference{
					APIVersion: fullSpec.Sink.APIVersion,
					Kind:       fullSpec.Sink.Kind,
					Namespace:  fullSpec.Sink.Namespace,
					Name:       "some-other-name",
				},
				ServiceAccountName: fullSpec.ServiceAccountName,
			},
			allowed: false,
		},
		"no change": {
			orig:    &fullSpec,
			updated: fullSpec,
			allowed: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			var orig *KafkaSource
			if tc.orig != nil {
				orig = &KafkaSource{
					Spec: *tc.orig,
				}
			}
			updated := &KafkaSource{
				Spec: tc.updated,
			}
			err := updated.CheckImmutableFields(context.TODO(), orig)
			if tc.allowed != (err == nil) {
				t.Fatalf("Unexpected immutable field check. Expected %v. Actual %v", tc.allowed, err)
			}
		})
	}
}
