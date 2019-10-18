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

package adapter

import (
	"context"
	"io/ioutil"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	_ "github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
)

type envConfig struct {
	adapter.EnvConfig

	CouchDbCredentialsPath string `envconfig:"COUCHDB_CREDENTIALS" required:"true"`
	Database               string `envconfig:"COUCHDB_DATABASE" required:"true"`
	EventSource            string `envconfig:"EVENT_SOURCE" required:"true"`
}

type couchDbAdapter struct {
	namespace string
	ce        cloudevents.Client
	reporter  source.StatsReporter
	logger    *zap.SugaredLogger

	source  string
	couchDB *kivik.DB
	options kivik.Options
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// NewAdapter creates an adapter to convert incoming CouchDb changes events to CloudEvents and
// then sends them to the specified Sink
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	rawurl, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/url")
	if err != nil {
		logger.Fatal("Missing url key in secret", zap.Error(err))
	}
	return newAdapter(ctx, env, ceClient, reporter, string(rawurl), "couch")
}

func newAdapter(ctx context.Context, env *envConfig, ceClient cloudevents.Client, reporter source.StatsReporter, url string, driver string) adapter.Adapter {
	logger := logging.FromContext(ctx)

	client, err := kivik.New(driver, url)
	if err != nil {
		logger.Fatal("Error creating connection to couchDB", zap.Error(err))
	}

	db := client.DB(context.TODO(), env.Database)
	if db.Err() != nil {
		logger.Fatal("Error connection to couchDB database", zap.Any("dabase", env.Database), zap.Error(err))
	}

	return &couchDbAdapter{
		namespace: env.Namespace,
		ce:        ceClient,
		reporter:  reporter,
		logger:    logger,

		couchDB: db,
		source:  env.EventSource,
		options: map[string]interface{}{
			"feed":  "normal",
			"since": "0",
		},
	}
}

func (a *couchDbAdapter) Start(stopCh <-chan struct{}) error {
	// Just Poll for now. Other modes (long running, continues) will be provided later on.
	return wait.PollUntil(2*time.Second, a.send, stopCh)
}

func (a *couchDbAdapter) send() (done bool, err error) {
	changes, err := a.couchDB.Changes(context.TODO(), a.options)
	if err != nil {
		return false, err
	}

	for changes.Next() {
		event, err := a.makeEvent(changes)
		if err != nil {
			return false, err
		}

		if _, _, err := a.ce.Send(context.Background(), *event); err != nil {
			a.logger.Info("event delivery failed", zap.Error(err))
			return false, err
		}

		a.options["since"] = changes.Seq()
	}

	return false, nil
}

func (a *couchDbAdapter) makeEvent(changes *kivik.Changes) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID(changes.Seq())
	event.SetSource(a.source)
	event.SetSubject(changes.ID())
	event.SetType(v1alpha1.CouchDbSourceChangesEventType)

	if err := event.SetData(changes.Changes()); err != nil {
		return nil, err
	}
	return &event, nil
}
