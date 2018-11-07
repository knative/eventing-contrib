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
