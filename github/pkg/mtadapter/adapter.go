/*
Copyright 2020 The Knative Authors

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

package mtadapter

import (
	"context"
	"fmt"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	githubsourceinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	"knative.dev/eventing-contrib/github/pkg/common"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing the HTTP port
	EnvPort string `envconfig:"PORT" default:"8080"`
}

// New EnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// gitHubAdapter converts incoming GitHub webhook events to CloudEvents
type gitHubAdapter struct {
	logger *zap.SugaredLogger
	client cloudevents.Client
	port   string

	router     *Router
	controller *controller.Impl
}

// NewAdapter returns the instance of gitHubReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	// Setting up the server receiving GitHubEvent
	lister := githubsourceinformer.Get(ctx).Lister()
	router := NewRouter(logger, lister, ceClient)

	return &gitHubAdapter{
		logger: logger,
		client: ceClient,
		port:   env.EnvPort,

		controller: NewController(ctx, router),
		router:     router,
	}
}

// Start implements adapter.Adapter
func (a *gitHubAdapter) Start(ctx context.Context) error {
	a.logger.Info("Starting controllers...")
	go controller.StartAll(ctx, a.controller)

	// Start our multi-tenant server receiving GitHub events
	server := &http.Server{
		Addr:    ":" + a.port,
		Handler: a.router,
	}

	done := make(chan bool, 1)
	go common.GracefulShutdown(server, a.logger, ctx.Done(), done)

	a.logger.Infof("Server is ready to handle requests at %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", server.Addr, err)
	}

	<-done
	a.logger.Infof("Server stopped")
	return nil
}
