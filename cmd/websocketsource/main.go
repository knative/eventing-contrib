/*
Copyright 2018 The Knative Authors

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
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/knative/pkg/cloudevents"
	"github.com/pkg/errors"
)

var (
	sink   string
	source string

	// CloudEvents specific parameters
	eventType   string
	eventSource string

	httpClient = &http.Client{}
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to send messages to")
	flag.StringVar(&source, "source", "", "the url to get messages from")
	flag.StringVar(&eventType, "eventType", "websocket-event", "the event-type (CloudEvents)")
	flag.StringVar(&eventSource, "eventSource", "", "the event-source (CloudEvents)")
}

func main() {
	flag.Parse()

	// "source" flag must not be empty for operation.
	if source == "" {
		log.Fatal("A valid source url must be defined.")
	}

	// The event's source defaults to the URL of where it was taken from.
	if eventSource == "" {
		eventSource = source
	}

	c, _, err := websocket.DefaultDialer.Dial(source, nil)
	if err != nil {
		log.Fatal("error connecting:", err)
	}

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			// TODO(markusthoemmes): Handle failures and reconnect
			log.Println("error while reading message:", err)
			return
		}
		if err := postMessage(message); err != nil {
			log.Printf("sending event to channel failed: %v", err)
		}
	}
}

func postMessage(message []byte) error {
	ctx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          eventType,
		EventID:            uuid.New().String(),
		Source:             eventSource,
		EventTime:          time.Now(),
	}

	req, err := cloudevents.Binary.NewRequest(sink, message, ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create http request")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}

	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed: code: %d, body: %s", resp.StatusCode, string(body))
	}

	io.Copy(ioutil.Discard, resp.Body)
	return nil
}
