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
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	BrokerConfigMapKey    = "bootstrapServers"
	KafkaChannelSeparator = "."

	// DefaultNumPartitions defines the default number of partitions
	DefaultNumPartitions = 1

	// DefaultReplicationFactor defines the default number of replications
	DefaultReplicationFactor = 1

	knativeKafkaTopicPrefix = "knative-messaging-kafka"

	DefaultMaxIdleConns        = 1000
	DefaultMaxIdleConnsPerHost = 100
)

var (
	firstKafkaConfigMapCall = true
)

type KafkaConfig struct {
	Brokers             []string
	MaxIdleConns        int
	MaxIdleConnsPerHost int
}

// GetKafkaConfig returns the details of the Kafka cluster.
func GetKafkaConfig(configMap map[string]string) (*KafkaConfig, error) {
	if len(configMap) == 0 {
		return nil, fmt.Errorf("missing configuration")
	}

	config := &KafkaConfig{}

	if brokers, ok := configMap[BrokerConfigMapKey]; ok {
		bootstrapServers := strings.Split(brokers, ",")
		for _, s := range bootstrapServers {
			if len(s) == 0 {
				return nil, fmt.Errorf("empty %s value in configuration", BrokerConfigMapKey)
			}
		}
		config.Brokers = bootstrapServers
	} else {
		return nil, fmt.Errorf("missing key %s in configuration", BrokerConfigMapKey)
	}

	if maxConns, ok := configMap["maxIdleConns"]; ok {
		mc, err := strconv.Atoi(maxConns)
		if err != nil {
			config.MaxIdleConns = DefaultMaxIdleConns
		}
		config.MaxIdleConns = mc
	} else {
		config.MaxIdleConns = DefaultMaxIdleConns
	}
	if maxConnsPerHost, ok := configMap["maxIdleConnsPerHost"]; ok {
		mcph, err := strconv.Atoi(maxConnsPerHost)
		if err != nil {
			config.MaxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
		}
		config.MaxIdleConnsPerHost = mcph

	} else {
		config.MaxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
	}

	return config, nil
}

func TopicName(separator, namespace, name string) string {
	topic := []string{knativeKafkaTopicPrefix, namespace, name}
	return strings.Join(topic, separator)
}

// We skip the first call into KafkaConfigMapObserver because it is not an indication
// of change of the watched ConfigMap but the map's initial state. See the comment for
// knative.dev/pkg/configmap/watcher.Start()
func KafkaConfigMapObserver(logger *zap.SugaredLogger) func(configMap *corev1.ConfigMap) {
	return func(kafkaConfigMap *corev1.ConfigMap) {
		if firstKafkaConfigMapCall {
			firstKafkaConfigMapCall = false
		} else {
			logger.Info("Kafka broker configuration updated, restarting")
			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}
	}
}
