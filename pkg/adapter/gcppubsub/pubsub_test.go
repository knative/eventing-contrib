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

package gcppubsub

import (
	"encoding/json"
	"testing"
	"time"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
)

type PubSubMockMessage struct {
	MockID          string
	MockPublishTime time.Time
	MockData        []byte

	Acked  bool
	Nacked bool

	M *pubsub.Message
}

func (t *PubSubMockMessage) Ack() {
	t.Acked = true
}

func (t *PubSubMockMessage) Nack() {
	t.Nacked = true
}

func (t *PubSubMockMessage) Data() []byte {
	return t.MockData
}

func (t *PubSubMockMessage) Message() *pubsub.Message {
	return t.M
}

func (t *PubSubMockMessage) ID() string {
	return t.MockID
}

func (t *PubSubMockMessage) PublishTime() time.Time {
	return t.MockPublishTime
}

func TestPubSubMessageWrapper(t *testing.T) {
	data, err := json.Marshal(map[string]string{"key": "value"})
	if err != nil {
		t.Errorf("unexpected error, %v", err)
	}
	now := time.Now()

	m := &PubSubMessageWrapper{
		M: &pubsub.Message{
			ID:          "ABC",
			Data:        data,
			PublishTime: now,
		},
	}

	if m.ID() != "ABC" {
		t.Errorf("expected ID to be %s, got %s", "ABC", m.ID())
	}

	if m.PublishTime() != now {
		t.Errorf("expected PublishTime to be %v, got %v", now, m.PublishTime())
	}

	if string(m.Data()) != string(data) {
		t.Errorf("expected Data to be %v, got %v", string(data), m.Data())
	}

	if m.Message() != m.M {
		t.Errorf("expected Message to be %v, got %v", m.M, m.Message())
	}
}
