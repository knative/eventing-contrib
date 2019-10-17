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

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/couchdb/source/pkg/couchdb"
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

	source     string
	couchDB    couchdb.Connection
	database   string
	changeArgs couchdb.ChangeArguments
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	rawurl, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/url")
	if err != nil {
		logger.Fatal("Missing url key in secret", zap.Error(err))
	}

	rawusername, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/username")
	if err != nil {
		logger.Fatal("Missing username key in secret", zap.Error(err))
	}

	rawpassword, err := ioutil.ReadFile(env.CouchDbCredentialsPath + "/password")
	if err != nil {
		logger.Fatal("Missing username key in secret", zap.Error(err))
	}

	connection, err := couchdb.Connect(string(rawurl), string(rawusername), string(rawpassword), time.Minute)
	if err != nil {
		logger.Fatal("Error creating connection to couchDb", zap.Error(err))
	}

	return &couchDbAdapter{
		namespace: env.Namespace,
		ce:        ceClient,
		reporter:  reporter,
		logger:    logger,

		couchDB:    connection,
		source:     env.EventSource,
		database:   env.Database,
		changeArgs: couchdb.ChangeArguments{},
	}
}

func (a *couchDbAdapter) Start(stopCh <-chan struct{}) error {
	// Just Poll for now. Other modes (long running, continues) will be provided later on.
	return wait.PollUntil(2*time.Second, a.send, stopCh)
}

func (a *couchDbAdapter) send() (done bool, err error) {
	changes, err := a.couchDB.Changes(a.database, &a.changeArgs)
	if err != nil {
		return false, err
	}

	if len(changes.Results) > 0 {
		for _, r := range changes.Results {

			event, err := a.makeEvent(&r)
			if err != nil {
				return false, err
			}

			if _, _, err := a.ce.Send(context.Background(), *event); err != nil {
				a.logger.Info("event delivery failed", zap.Error(err))
				return false, err
			}

			a.changeArgs.Since = r.Seq
		}
	}

	return false, nil
}

func (a *couchDbAdapter) makeEvent(changes *couchdb.ChangeResponseResult) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID(changes.ID)
	event.SetSource(a.source)
	event.SetType(v1alpha1.CouchDbSourceChangesEventType)

	if err := event.SetData(changes); err != nil {
		return nil, err
	}
	return &event, nil
}
