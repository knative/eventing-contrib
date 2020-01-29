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

package resources

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
	_ "knative.dev/pkg/metrics/testing"
)

func TestMakeReceiveAdapter(t *testing.T) {
	name := "source-name"
	src := &v1alpha1.CouchDbSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "source-namespace",
			UID:       "1234",
		},
		Spec: v1alpha1.CouchDbSourceSpec{
			ServiceAccountName: "source-svc-acct",
			Database:           "mydb",
			Feed:               v1alpha1.FeedContinuous,
		},
	}

	got := MakeReceiveAdapter(&ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SinkURI: "sink-uri",
	})

	one := int32(1)
	trueValue := true

	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "source-namespace",
			Name:      fmt.Sprintf("couchdbsource-%s-1234", name),
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "sources.knative.dev/v1alpha1",
					Kind:               "CouchDbSource",
					Name:               name,
					UID:                "1234",
					Controller:         &trueValue,
					BlockOwnerDeletion: &trueValue,
				},
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "source-svc-acct",
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: "test-image",
							Env: []corev1.EnvVar{
								{
									Name:  "SINK_URI",
									Value: "sink-uri",
								}, {
									Name:  "EVENT_SOURCE",
									Value: "",
								}, {
									Name:  "COUCHDB_CREDENTIALS",
									Value: "/etc/couchdb-credentials",
								}, {
									Name:  "COUCHDB_DATABASE",
									Value: "mydb",
								}, {
									Name:  "COUCHDB_FEED",
									Value: "continuous",
								}, {
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								}, {
									Name:  "METRICS_DOMAIN",
									Value: "knative.dev/eventing",
								}, {
									Name:  "K_METRICS_CONFIG",
									Value: "",
								}, {
									Name:  "K_LOGGING_CONFIG",
									Value: "",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "couchdb-credentials",
									MountPath: "/etc/couchdb-credentials",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{{
						Name: "couchdb-credentials",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{}}}},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
