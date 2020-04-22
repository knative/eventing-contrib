/*
Copyright 2020 The Knative Authors

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
	"context"

	"knative.dev/eventing-contrib/github/pkg/adapter"
	"knative.dev/pkg/metrics"

	"go.uber.org/zap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/signals"
)

const (
	component = "githubsource-adapter"
)

func main() {
	// This code is the same as sharedmain.Main except:
	// - no need to start the profiling server since ksvc are HA
	// - start our github server
	// - temporary: the metrics server is not started due to http port conflict

	ctx := signals.NewContext()
	cfg := sharedmain.ParseAndGetConfigOrDie()

	// TODO: metrics
	//sharedmain.MemStatsOrDie(ctx)

	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, component)
	defer flush(logger)
	ctx = logging.WithLogger(ctx, logger)

	sharedmain.CheckK8sClientMinimumVersionOrDie(ctx, logger)

	cmw := sharedmain.SetupConfigMapWatchOrDie(ctx, logger)

	handler := adapter.NewHandler(logger)
	ctx = adapter.WithHandler(ctx, handler)

	ra := adapter.NewController(ctx, cmw)

	sharedmain.WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)

	// TODO: metrics
	//sharedmain.WatchObservabilityConfigOrDie(ctx, cmw, profilingHandler, logger, component)

	logger.Info("Starting configuration manager...")
	if err := cmw.Start(ctx.Done()); err != nil {
		logger.Fatalw("Failed to start configuration manager", zap.Error(err))
	}
	logger.Info("Starting informers...")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		logger.Fatalw("Failed to start informers", zap.Error(err))
	}
	logger.Info("Starting controllers...")
	go controller.StartAll(ctx.Done(), ra)

	// Start our GitHub webhook server
	server := adapter.NewServer(handler)

	go func() {
		server.ListenAndServe()
	}()

	<-ctx.Done()

	logger.Info("Shutting down webhook server")
	server.Shutdown(context.Background())
}

func flush(logger *zap.SugaredLogger) {
	logger.Sync()
	metrics.FlushExporter()
}
