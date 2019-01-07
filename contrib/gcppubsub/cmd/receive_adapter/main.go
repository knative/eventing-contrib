/*
Copyright 2018 The Knative Authors

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

	gcppubsub "github.com/knative/eventing-sources/contrib/gcppubsub/pkg/adapter"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"golang.org/x/net/context"
)

const (
	// Environment variable containing project id
	envProject = "GCPPUBSUB_PROJECT"

	// Sink for messages.
	envSinkURI = "SINK_URI"

	// envTopic is the name of the environment variable that contains the GCP PubSub Topic being
	// subscribed to's name. In the form that is unique within the project. E.g. 'laconia', not
	// 'projects/my-gcp-project/topics/laconia'.
	envTopic = "GCPPUBSUB_TOPIC"

	// Name of the subscription to use
	envSubscription = "GCPPUBSUB_SUBSCRIPTION_ID"
)

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
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

	adapter := &gcppubsub.Adapter{
		ProjectID:      getRequiredEnv(envProject),
		TopicID:        getRequiredEnv(envTopic),
		SinkURI:        getRequiredEnv(envSinkURI),
		SubscriptionID: getRequiredEnv(envSubscription),
	}

	logger.Info("Starting GCP Pub/Sub Receive Adapter. %v", zap.Reflect("adapter", adapter))
	if err := adapter.Start(ctx); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}
