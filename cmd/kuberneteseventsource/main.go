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
	"context"
	"flag"
	"log"

	"github.com/knative/eventing-sources/pkg/adapter/kubernetesevents"
	"go.uber.org/zap"
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
	ctx := context.Background()
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	if namespace == "" {
		logger.Fatal("No namespace provided")
	}

	if sink == "" {
		logger.Fatal("No sink provided")
	}

	a := kubernetesevents.Adapter{
		Namespace: namespace,
		SinkURI:   sink,
	}

	a.Run(ctx)

	log.Printf("Exiting...")
}
