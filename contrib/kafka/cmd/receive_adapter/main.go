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

package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	kafka "github.com/knative/eventing-sources/contrib/kafka/pkg/adapter"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"golang.org/x/net/context"

	"github.com/knative/pkg/signals"
)

const (
	envBootstrapServers = "KAFKA_BOOTSTRAP_SERVERS"
	envTopics           = "KAFKA_TOPICS"
	envConsumerGroup    = "KAFKA_CONSUMER_GROUP"
	envNetSASLEnable    = "KAFKA_NET_SASL_ENABLE"
	envNetSASLUser      = "KAFKA_NET_SASL_USER"
	envNetSASLPassword  = "KAFKA_NET_SASL_PASSWORD"
	envNetTLSEnable     = "KAFKA_NET_TLS_ENABLE"
	envSinkURI          = "SINK_URI"
)

func getRequiredEnv(key string) string {
	val, defined := os.LookupEnv(key)
	if !defined {
		log.Fatalf("Required environment variable not defined for key '%s'.", key)
	}

	return val
}

func getOptionalBoolEnv(key string) bool {
	if val, defined := os.LookupEnv(key); defined {
		if res, err := strconv.ParseBool(val); err != nil {
			log.Fatalf("A value of '%s' cannot be parsed as a boolean value.", val)
		} else {
			return res
		}
	}

	return false
}

func main() {
	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	adapter := &kafka.Adapter{
		BootstrapServers: getRequiredEnv(envBootstrapServers),
		Topics:           getRequiredEnv(envTopics),
		ConsumerGroup:    getRequiredEnv(envConsumerGroup),
		SinkURI:          getRequiredEnv(envSinkURI),
		Net: kafka.AdapterNet{
			SASL: kafka.AdapterSASL{
				Enable:   getOptionalBoolEnv(envNetSASLEnable),
				User:     os.Getenv(envNetSASLUser),
				Password: os.Getenv(envNetSASLPassword),
			},
			TLS: kafka.AdapterTLS{
				Enable: getOptionalBoolEnv(envNetTLSEnable),
			},
		},
	}

	stopCh := signals.SetupSignalHandler()

	logger.Info("Starting Apache Kafka Receive Adapter...", zap.Reflect("adapter", adapter))
	if err := adapter.Start(ctx, stopCh); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}
