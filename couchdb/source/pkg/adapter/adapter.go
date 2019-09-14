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
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/couchdb/source/pkg/couchdb"
)

// Hold options for the adapter
type Options struct {
	EventSource string
	CouchDbURL  string
	Username    string
	Password    string
	Database    string
	Namespace   string
}

// Adapter converts incoming CouchDb events to CloudEvents
type Adapter interface {
	Start(stopCh <-chan struct{}) error
}

type adapter struct {
	source     string
	ce         cloudevents.Client
	namespace  string
	logger     *zap.SugaredLogger
	couchDB    couchdb.Connection
	database   string
	changeArgs couchdb.ChangeArguments
}

// New creates an adapter to convert incoming CouchDb changes events to CloudEvents and
// then sends them to the specified Sink
func New(ceClient cloudevents.Client, logger *zap.SugaredLogger, options *Options) (Adapter, error) {
	connection, err := couchdb.Connect(options.CouchDbURL, options.Username, options.Password, time.Minute)
	if err != nil {
		return nil, err
	}

	a := &adapter{
		ce:         ceClient,
		logger:     logger,
		namespace:  options.Namespace,
		couchDB:    connection,
		source:     options.EventSource,
		database:   options.Database,
		changeArgs: couchdb.ChangeArguments{},
	}

	return a, nil
}

func (a *adapter) Start(stopCh <-chan struct{}) error {
	// Just Poll for now. Other modes (long running, continues) will be provided later on.
	return wait.PollUntil(2*time.Second, a.send, stopCh)
}

func (a *adapter) send() (done bool, err error) {
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

func (a *adapter) makeEvent(changes *couchdb.ChangeResponseResult) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent()
	event.SetID(changes.ID)
	event.SetSource(a.source)
	event.SetType(v1alpha1.CouchDbSourceChangesEventType)

	if err := event.SetData(changes); err != nil {
		return nil, err
	}
	return &event, nil
}
