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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/logging"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/configmap"
)

const (
	BrokerConfigMapKey           = "bootstrapServers"
	AuthSecretName               = "authSecretName"
	AuthSecretNamespace          = "authSecretNamespace"
	MaxIdleConnectionsKey        = "maxIdleConns"
	MaxIdleConnectionsPerHostKey = "maxIdleConnsPerHost"

	TlsCacert    = "ca.crt"
	TlsUsercert  = "user.crt"
	TlsUserkey   = "user.key"
	SaslUser     = "user"
	SaslPassword = "password"
	SaslType     = "saslType"

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
	AuthSecretName      string
	AuthSecretNamespace string
}

type KafkaAuthConfig struct {
	TLS  *KafkaTlsConfig
	SASL *KafkaSaslConfig
}

type KafkaTlsConfig struct {
	Cacert   string
	Usercert string
	Userkey  string
}

type KafkaSaslConfig struct {
	User     string
	Password string
	SaslType string
}

func GetKafkaAuthData(ctx context.Context, secretname string, secretNS string) *KafkaAuthConfig {

	k8sClient := kubeclient.Get(ctx)
	secret, err := k8sClient.CoreV1().Secrets(secretNS).Get(ctx, secretname, metav1.GetOptions{})

	if err != nil || secret == nil {
		logging.FromContext(ctx).Errorf("Referenced Auth Secret not found")
		return nil
	}

	kafkaAuthConfig := &KafkaAuthConfig{}
	// check for TLS
	if string(secret.Data[TlsCacert]) != "" {
		tls := &KafkaTlsConfig{
			Cacert:   string(secret.Data[TlsCacert]),
			Usercert: string(secret.Data[TlsUsercert]),
			Userkey:  string(secret.Data[TlsUserkey]),
		}
		kafkaAuthConfig.TLS = tls
	}

	if string(secret.Data[SaslUser]) != "" {
		sasl := &KafkaSaslConfig{
			User:     string(secret.Data[SaslUser]),
			Password: string(secret.Data[SaslPassword]),
			SaslType: string(secret.Data[SaslType]),
		}
		kafkaAuthConfig.SASL = sasl
	}
	return kafkaAuthConfig
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
	var authSecretNamespace string
	var authSecretName string

	err := configmap.Parse(configMap,
		configmap.AsString(BrokerConfigMapKey, &bootstrapServers),
		configmap.AsString(AuthSecretName, &authSecretName),
		configmap.AsString(AuthSecretNamespace, &authSecretNamespace),
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
	config.AuthSecretName = authSecretName
	config.AuthSecretNamespace = authSecretNamespace

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
