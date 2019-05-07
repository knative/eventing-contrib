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

package gcppubsub

import (
	"fmt"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"golang.org/x/net/context"
)

// Transformer implements the GCP Pub/Sub transformer that transforms Pub/Sub messages wrapped in
// CloudEvents payloads into CloudEvents.
type Transformer struct {
	ceClient cloudevents.Client
}

func (t *Transformer) Start(ctx context.Context) error {
	var err error
	if t.ceClient == nil {
		if t.ceClient, err = kncloudevents.NewDefaultClient(); err != nil {
			return fmt.Errorf("failed to create cloudevent client: %s", err.Error())
		}
	}

	// Using that ceClient, start receiving CloudEvents.
	// Note: ceClient.StartReceiver is a blocking call.
	return t.ceClient.StartReceiver(ctx, func(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
		return t.transformEvent(ctx, event, resp)
	})
}

func (t *Transformer) transformEvent(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	logger := logging.FromContext(ctx).With(zap.Any("eventID", event.ID()))
	logger.Debugw("Received wrapped PubSub message in cloudEvent", zap.String("cloudEvent", event.String()))

	// TODO transform
	resp.Event = &event
	return nil
}
