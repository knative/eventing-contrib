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

type PubSubMessage interface {
	Ack()
	Nack()
	ID() string
	PublishTime() time.Time
	Data() []byte

	Message() *pubsub.Message
}

type PubSubMessageWrapper struct {
	M *pubsub.Message
}

func (w *PubSubMessageWrapper) Ack() {
	w.M.Ack()
}

func (w *PubSubMessageWrapper) Nack() {
	w.M.Nack()
}

func (w *PubSubMessageWrapper) Data() []byte {
	return w.M.Data
}

func (w *PubSubMessageWrapper) Message() *pubsub.Message {
	return w.M
}

func (w *PubSubMessageWrapper) ID() string {
	return w.M.ID
}

func (w *PubSubMessageWrapper) PublishTime() time.Time {
	return w.M.PublishTime
}
