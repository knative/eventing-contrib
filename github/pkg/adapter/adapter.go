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

package adapter

import (
	"context"
	"fmt"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"

	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/github/pkg/common"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing GitHub secret token
	EnvSecret string `envconfig:"GITHUB_SECRET_TOKEN" required:"true"`
	// Environment variable containing the HTTP port
	EnvPort string `envconfig:"PORT" default:"8080"`
	// Environment variable containing information about the origin of the event
	EnvOwnerRepo string `envconfig:"GITHUB_OWNER_REPO" required:"true"`
}

// NewEnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// gitHubAdapter converts incoming GitHub webhook events to CloudEvents
type gitHubAdapter struct {
	logger *zap.SugaredLogger
	client cloudevents.Client
	source string

	secretToken string
	port        string
}

// NewAdapter returns the instance of gitHubReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	return &gitHubAdapter{
		logger:      logger,
		client:      ceClient,
		port:        env.EnvPort,
		secretToken: env.EnvSecret,
		source:      sourcesv1alpha1.GitHubEventSource(env.EnvOwnerRepo),
	}
}

func (a *gitHubAdapter) Start(ctx context.Context) error {
	src := cloudevents.ParseURIRef(a.source)
	if src == nil {
		return fmt.Errorf("invalid source for github events: %s", a.source)
	}
	done := make(chan bool, 1)

	server := &http.Server{
		Addr:    ":" + a.port,
		Handler: a.newRouter(),
	}

	go common.GracefulShutdown(server, a.logger, ctx.Done(), done)

	a.logger.Infof("Server is ready to handle requests at %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", server.Addr, err)
	}

	<-done
	a.logger.Infof("Server stopped")
	return nil
}

func (a *gitHubAdapter) newRouter() http.Handler {
	router := http.NewServeMux()
	handler := common.NewHandler(a.client, "", a.source, a.secretToken, a.logger)
	router.Handle("/", handler)
	return router
}
