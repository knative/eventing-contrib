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

package cronjobevents

import (
	"context"
	"encoding/json"

	"github.com/robfig/cron"

	"github.com/knative/pkg/cloudevents"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
)

const (
	eventType = "dev.knative.cronjob.event"
)

// Adapter implements the Cron Job adapter to trigger a Sink.
type Adapter struct {
	// Schedule is a cron format string such as 0 * * * * or @hourly
	Schedule string

	// Data is the data to be posted to the target.
	Data string

	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	// client sends cloudevents.
	client *cloudevents.Client
}

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)

	sched, err := cron.ParseStandard(a.Schedule)
	if err != nil {
		logger.Error("Unparseable schedule: ", a.Schedule, zap.Error(err))
		return err
	}
	c := cron.New()
	c.Schedule(sched, cron.FuncJob(a.cronTick))
	c.Start()
	<-stopCh
	c.Stop()
	logger.Info("Shutting down.")
	return nil
}

func (a *Adapter) cronTick() {
	logger := logging.FromContext(context.TODO())
	if a.client == nil {
		a.client = cloudevents.NewClient(a.SinkURI, cloudevents.Builder{
			EventType: eventType,
			Source:    "CronJob",
		})
	}
	if err := a.client.Send(message(a.Data)); err != nil {
		logger.Error("failed to send cloudevent", err)
	}
}

type Message struct {
	Body string `json:"body"`
}

func message(body string) interface{} {
	// try to marshal the body into an interface.
	var objmap map[string]*json.RawMessage
	if err := json.Unmarshal([]byte(body), &objmap); err != nil {
		//default to a wrapped message.
		return Message{Body: body}
	}
	return objmap
}
