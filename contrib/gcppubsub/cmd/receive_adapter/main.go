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
	"github.com/kelseyhightower/envconfig"
	"github.com/knative/eventing-sources/contrib/gcppubsub/pkg/adapter"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"golang.org/x/net/context"
)

type envConfig struct {
	// Environment variable containing project id.
	Project string `envconfig:"GCPPUBSUB_PROJECT" required:"true"`

	// Environment variable containing the sink URI.
	Sink string `envconfig:"SINK_URI" required:"true"`

	// Environment variable containing the transformer URI.
	Transformer string `envconfig:"TRANSFORMER_URI" default:""`

	// Environment variable containing the GCP PubSub Topic being
	// subscribed to's name. In the form that is unique within the project. E.g. 'laconia', not
	// 'projects/my-gcp-project/topics/laconia'.
	Topic string `envconfig:"GCPPUBSUB_TOPIC" default:""`

	// Environment variable containing the name of the subscription to use.
	Subscription string `envconfig:"GCPPUBSUB_SUBSCRIPTION_ID" required:"true"`
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

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatal("Failed to process env var", zap.Error(err))
	}

	adapter := &gcppubsub.Adapter{
		ProjectID:      env.Project,
		TopicID:        env.Topic,
		SinkURI:        env.Sink,
		SubscriptionID: env.Subscription,
		TransformerURI: env.Transformer,
	}

	logger.Info("Starting GCP Pub/Sub Receive Adapter. %v", zap.Reflect("adapter", adapter))
	if err := adapter.Start(ctx); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}
