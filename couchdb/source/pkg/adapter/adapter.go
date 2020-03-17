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
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"github.com/go-kivik/couchdb"
	"github.com/go-kivik/kivik"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
)

type envConfig struct {
	adapter.EnvConfig

	CouchDbCredentialsPath string `envconfig:"COUCHDB_CREDENTIALS" required:"true"`
	Database               string `envconfig:"COUCHDB_DATABASE" required:"true"`
	EventSource            string `envconfig:"EVENT_SOURCE" required:"true"`
	Feed                   string `envconfig:"COUCHDB_FEED" required:"true"`
}

type couchDbAdapter struct {
	namespace string
	ce        cloudevents.Client
	reporter  source.StatsReporter
	logger    *zap.SugaredLogger

	source  string
	feed    string
	couchDB *kivik.DB
	options kivik.Options
}

func init() {
	// Need to disable compression for Cloudant.
	var transport = http2.Transport{
		DisableCompression: true,
	}
	kivik.Register("cloudant", &couchdb.Couch{
		HTTPClient: &http.Client{Transport: &transport},
	})
}

// NewEnvConfig creates an empty configuration
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
	url := string(rawurl)

	driver := "couch"

	// Use cloudant driver only when the server is Cloudant.
	if strings.Contains(url, "cloudant") {
		driver = "cloudant"
	}

	return newAdapter(ctx, env, ceClient, reporter, url, driver)
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
		feed:    env.Feed,
		options: map[string]interface{}{
			"feed":  env.Feed,
			"since": "0",
		},
	}
}

func (a *couchDbAdapter) Start(stopCh <-chan struct{}) error {
	period := 2 * time.Second
	if a.feed == "continuous" {
		a.options["heartbeat"] = 6000
	}
	wait.Until(a.processChanges, period, stopCh)
	return nil
}

func (a *couchDbAdapter) processChanges() {
	changes, err := a.couchDB.Changes(context.TODO(), a.options)
	if err != nil {
		a.logger.Error("Error getting the list of changes", zap.Error(err))
		return
	}

	for changes.Next() {
		if changes.Seq() != "" {
			event, err := a.makeEvent(changes)

			if err != nil {
				a.logger.Error("error making event", zap.Error(err))
			}

			if _, _, err := a.ce.Send(context.TODO(), *event); err != nil {
				a.logger.Error("event delivery failed", zap.Error(err))
			}

			a.options["since"] = changes.Seq()
		}
	}

	if changes.Err() != nil {
		if changes.Err() == io.EOF {
			a.logger.Error("The connection to the changes feed was interrupted.", zap.Error(changes.Err()))
		} else {
			a.logger.Error("Error found in the changes feed.", zap.Error(changes.Err()))
		}
	}
}

func (a *couchDbAdapter) makeEvent(changes *kivik.Changes) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(changes.Seq())
	event.SetSource(a.source)
	event.SetSubject(changes.ID())
	event.SetDataContentType(cloudevents.ApplicationJSON)

	if changes.Deleted() {
		event.SetType(v1alpha1.CouchDbSourceDeleteEventType)
	} else {
		event.SetType(v1alpha1.CouchDbSourceUpdateEventType)
	}

	if err := event.SetData(changes.Changes()); err != nil {
		return nil, err
	}
	return &event, nil
}
