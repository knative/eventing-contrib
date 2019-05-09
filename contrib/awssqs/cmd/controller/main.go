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
	"log"

	"github.com/knative/eventing-sources/contrib/awssqs/pkg/apis"
	"github.com/knative/eventing-sources/contrib/awssqs/pkg/reconciler"
	"github.com/knative/pkg/logging/logkey"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {

	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatal(err)
	}
	logger = logger.With(zap.String(logkey.ControllerType, "awssqs-controller"))

	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatal("Unable to get API server config: ", err)
	}

	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatal("Unable to create a new Manager", err)
	}

	log.Printf("Registering Components.")
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatal("Unable to setup Scheme for resources", err)
	}

	log.Printf("Setting up managers.")
	if err := reconciler.Add(mgr, logger.Sugar()); err != nil {
		log.Fatal("Unable to add Manager to the reconciler", err)
	}

	log.Printf("Starting AWS SQS source controller.")
	log.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
