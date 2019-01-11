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

	"github.com/knative/pkg/signals"

	"github.com/knative/eventing-sources/pkg/adapter/kubernetesevents"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sink      string
	namespace string
)

func init() {
	flag.StringVar(&sink, "sink", "", "uri to send events to")
	flag.StringVar(&namespace, "namespace", "default", "namespace to watch events for")
}

func main() {
	flag.Parse()

	logConfig := zap.NewProductionConfig()
	logConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logConfig.Build()

	if err != nil {
		log.Fatalf("unable to create logger: %v", err)
	}

	if namespace == "" {
		logger.Fatal("no namespace provided")
	}

	if sink == "" {
		logger.Fatal("no sink provided")
	}

	a := kubernetesevents.Adapter{
		Namespace: namespace,
		SinkURI:   sink,
	}

	stopCh := signals.SetupSignalHandler()

	err = a.Start(stopCh)
	if err != nil {
		logger.Fatal("Failed to start the adapter", zap.Error(err))
	}

	logger.Info("exiting...")
}
