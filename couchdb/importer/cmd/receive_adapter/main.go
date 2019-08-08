/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"io/ioutil"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"knative.dev/eventing-contrib/couchdb/importer/pkg/adapter"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/signals"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"
)

type envConfig struct {
	Namespace              string `envconfig:"SYSTEM_NAMESPACE" default:"default"`
	CouchDbCredentialsPath string `envconfig:"COUCHDB_CREDENTIALS" required:"true"`
	Database               string `envconfig:"COUCHDB_DATABASE" required:"true"`
	EventSource            string `envconfig:"EVENT_SOURCE" required:"true"`
	SinkURI                string `split_words:"true" required:"true"`
}

func main() {
	flag.Parse()

	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	dlogger, err := logCfg.Build()
	logger := dlogger.Sugar()

	var env envConfig
	err = envconfig.Process("", &env)
	if err != nil {
		logger.Fatalw("Error processing environment", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	logger = logger.With(zap.String("controller/couchdb", "adapter"))
	logger.Info("Starting the adapter")

	if err = tracing.SetupStaticZipkinPublishing("couchdbimporter", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the importer to continue to
		// start.
		logger.Error("Error setting up Zipkin publishing", zap.Error(err))
	}

	eventsClient, err := kncloudevents.NewDefaultClient(env.SinkURI)
	if err != nil {
		logger.Fatalw("Error building cloud event client", zap.Error(err))
	}

	// Reading secret keys
	rawurl, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/url")
	if err != nil {
		logger.Fatalf("Missing url key in secret")
	}

	rawusername, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/username")
	if err != nil {
		logger.Fatalf("Missing username key in secret")
	}

	rawpassword, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/password")
	if err != nil {
		logger.Fatalf("Missing username key in secret")
	}

	opt := adapter.Options{
		EventSource: env.EventSource,
		CouchDbURL:  string(rawurl),
		Username:    string(rawusername),
		Password:    string(rawpassword),
		Namespace:   env.Namespace,
		Database:    env.Database,
	}

	a, err := adapter.New(eventsClient, logger, &opt)
	if err != nil {
		logger.Fatalf("Failed to create couchdb adapter: %s", err.Error())
	}

	logger.Info("starting couchDB adapter")
	if err := a.Start(stopCh); err != nil {
		logger.Warn("start returned an error,", zap.Error(err))
	}
}
