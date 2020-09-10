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

package utils

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	_ "knative.dev/pkg/system/testing"
)

func TestGenerateTopicNameWithDot(t *testing.T) {
	expected := "knative-messaging-kafka.channel-namespace.channel-name"
	actual := TopicName(".", "channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestGenerateTopicNameWithHyphen(t *testing.T) {
	expected := "knative-messaging-kafka-channel-namespace-channel-name"
	actual := TopicName("-", "channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestGetKafkaConfig(t *testing.T) {

	testCases := []struct {
		name     string
		data     map[string]string
		getError string
		expected *KafkaConfig
	}{
		{
			name:     "invalid config path",
			data:     nil,
			getError: "missing configuration",
		},
		{
			name:     "configmap with no data",
			data:     map[string]string{},
			getError: "missing configuration",
		},
		{
			name:     "configmap with no bootstrapServers key",
			data:     map[string]string{"key": "value"},
			getError: "missing or empty key bootstrapServers in configuration",
		},
		{
			name:     "configmap with empty bootstrapServers value",
			data:     map[string]string{"bootstrapServers": ""},
			getError: "missing or empty key bootstrapServers in configuration",
		},
		{
			name: "single bootstrapServers",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "multiple bootstrapServers",
			data: map[string]string{"bootstrapServers": "kafkabroker1.kafka:9092,kafkabroker2.kafka:9092"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker1.kafka:9092", "kafkabroker2.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "partition consumer",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "partitions"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "default multiplex",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "multiplex"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "default multiplex from invalid consumerMode",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "foo"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "default multiplex from invalid consumerMode elevated max idle connections",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "foo", "maxIdleConns": "9000"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        9000,
				MaxIdleConnsPerHost: 100,
			},
		},
		{
			name: "default multiplex from invalid consumerMode elevated max idle connections per host",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "foo", "maxIdleConnsPerHost": "900"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 900,
			},
		},
		{
			name: "default multiplex from invalid consumerMode elevated max idle values",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "foo", "maxIdleConns": "9000", "maxIdleConnsPerHost": "600"},
			expected: &KafkaConfig{
				Brokers:             []string{"kafkabroker.kafka:9092"},
				MaxIdleConns:        9000,
				MaxIdleConnsPerHost: 600,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running %s", t.Name())

			got, err := GetKafkaConfig(tc.data)

			if tc.getError != "" {
				if err == nil {
					t.Errorf("Expected Config error: '%v'. Actual nil", tc.getError)
				} else if err.Error() != tc.getError {
					t.Errorf("Unexpected Config error. Expected '%v'. Actual '%v'", tc.getError, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected Config error. Expected nil. Actual '%v'", err)
			}

			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("unexpected Config (-want, +got) = %v", diff)
			}

		})
	}

}

func TestFindContainer(t *testing.T) {
	testCases := []struct {
		name          string
		deployment    *appsv1.Deployment
		containerName string
		expected      *corev1.Container
	}{
		{
			name: "no containers in deployment",
			deployment: &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{},
					},
				},
			}},
			containerName: "foo",
			expected:      nil,
		},
		{
			name: "no container found",
			deployment: &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "foo",
							Image: "example.registry.com/foo",
						}},
					},
				},
			}},
			containerName: "bar",
			expected:      nil,
		},
		{
			name: "container found",
			deployment: &appsv1.Deployment{Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "bar",
							Image: "example.registry.com/bar",
						}, {
							Name:  "foo",
							Image: "example.registry.com/foo",
						}},
					},
				},
			}},
			containerName: "foo",
			expected: &corev1.Container{
				Name:  "foo",
				Image: "example.registry.com/foo",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running %s", t.Name())

			got := FindContainer(tc.deployment, tc.containerName)

			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("unexpected container found (-want, +got) = %v", diff)
			}
		})
	}

}
