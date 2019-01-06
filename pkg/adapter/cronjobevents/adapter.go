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
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"github.com/robfig/cron"

	"github.com/knative/pkg/cloudevents"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/signals"
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
}

func (a *Adapter) Start(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	sched, err := cron.ParseStandard(a.Schedule)
	if err != nil {
		logger.Error("Unparseable schedule: ", a.Schedule, zap.Error(err))
		return err
	}
	c := cron.New()
	c.Schedule(sched, cron.FuncJob(func() {
		m := Message{ID: a.generateMessageId(), EventTime: time.Now().UTC(), Body: a.Data}
		logger := logger.With(zap.Any("eventID", m.ID), zap.Any("sink", a.SinkURI))
		err = a.postMessage(ctx, logger, &m)
		if err != nil {
			logger.Infof("Failed to post message: %s", err)
			return
		}
		logger.Debug("Finished posting message job.")
	}))
	stopCh := signals.SetupSignalHandler()
	c.Start()
	<-stopCh
	c.Stop()
	logger.Info("Shutting down.")
	return nil
}

func (a *Adapter) postMessage(ctx context.Context, logger *zap.SugaredLogger, m *Message) error {
	event := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          eventType,
		EventID:            m.ID,
		EventTime:          m.EventTime,
		Source:             "CronJob",
	}
	req, err := cloudevents.Binary.NewRequest(a.SinkURI, m, event)
	if err != nil {
		return fmt.Errorf("Failed to marshal message: %s", err)
	}

	logger.Debugw("Posting message")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debugw("Response", zap.String("status", resp.Status), zap.ByteString("body", body))

	// If the response is not within the 2xx range, return an error.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("[%d] unexpected response %s", resp.StatusCode, body)
	}

	return nil
}

func (a *Adapter) generateMessageId() string {
	return fmt.Sprintf("%v-%v", time.Now().UnixNano(), rand.Intn(900)+100)
}

type Message struct {
	ID        string
	EventTime time.Time
	Body      string
}
