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
	"context"
	"errors"
	"fmt"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"knative.dev/pkg/configmap"
)

const (
	BrokerConfigMapKey           = "bootstrapServers"
	MaxIdleConnectionsKey        = "maxIdleConns"
	MaxIdleConnectionsPerHostKey = "maxIdleConnsPerHost"
	KafkaAuthSecretName          = "kafka-broker-tls"

	KafkaChannelSeparator = "."

	// DefaultNumPartitions defines the default number of partitions
	DefaultNumPartitions = 1

	// DefaultReplicationFactor defines the default number of replications
	DefaultReplicationFactor = 1

	knativeKafkaTopicPrefix = "knative-messaging-kafka"

	DefaultMaxIdleConns        = 1000
	DefaultMaxIdleConnsPerHost = 100
)

type KafkaConfig struct {
	Brokers             []string
	MaxIdleConns        int32
	MaxIdleConnsPerHost int32
}

// GetKafkaConfig returns the details of the Kafka cluster.
func GetKafkaConfig(configMap map[string]string) (*KafkaConfig, error) {
	if len(configMap) == 0 {
		return nil, fmt.Errorf("missing configuration")
	}

	config := &KafkaConfig{
		MaxIdleConns:        DefaultMaxIdleConns,
		MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
	}

	var bootstrapServers string

	err := configmap.Parse(configMap,
		configmap.AsString(BrokerConfigMapKey, &bootstrapServers),
		configmap.AsInt32(MaxIdleConnectionsKey, &config.MaxIdleConns),
		configmap.AsInt32(MaxIdleConnectionsPerHostKey, &config.MaxIdleConnsPerHost),
	)
	if err != nil {
		return nil, err
	}

	if bootstrapServers == "" {
		return nil, errors.New("missing or empty key bootstrapServers in configuration")
	}
	bootstrapServersSplitted := strings.Split(bootstrapServers, ",")
	for _, s := range bootstrapServersSplitted {
		if len(s) == 0 {
			return nil, fmt.Errorf("empty %s value in configuration", BrokerConfigMapKey)
		}
	}
	config.Brokers = bootstrapServersSplitted

	return config, nil
}

func TopicName(separator, namespace, name string) string {
	topic := []string{knativeKafkaTopicPrefix, namespace, name}
	return strings.Join(topic, separator)
}

func FindContainer(d *appsv1.Deployment, containerName string) *corev1.Container {
	for i := range d.Spec.Template.Spec.Containers {
		if d.Spec.Template.Spec.Containers[i].Name == containerName {
			return &d.Spec.Template.Spec.Containers[i]
		}
	}

	return nil
}

// borrowed from eventing/pkg/util
func CopySecret(corev1Input clientcorev1.CoreV1Interface, srcNS string, srcSecretName string, tgtNS string) (*corev1.Secret, error) {
	srcSecrets := corev1Input.Secrets(srcNS)
	tgtNamespaceSecrets := corev1Input.Secrets(tgtNS)

	// First try to find the secret we're supposed to copy
	srcSecret, err := srcSecrets.Get(context.Background(), srcSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// check for nil source secret
	if srcSecret == nil {
		return nil, errors.New("error copying secret; there is no error but secret is nil")
	}

	// Found the secret, so now make a copy in our new namespace
	newSecret, err := tgtNamespaceSecrets.Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: KafkaAuthSecretName,
			},
			Data: srcSecret.Data,
			Type: srcSecret.Type,
		},
		metav1.CreateOptions{})

	// If the secret already exists then that's ok - may have already been created
	if err != nil && !apierrs.IsAlreadyExists(err) {
		return nil, fmt.Errorf("error copying the Secret: %s", err)
	}
	return newSecret, nil
}
