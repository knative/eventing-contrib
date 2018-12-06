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

package sources

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/knative/pkg/cloudevents"
)

var testCtx = cloudevents.EventContext{
	CloudEventsVersion: cloudevents.CloudEventsVersion,
	EventType:          "testType",
	EventID:            "testId",
	Source:             "testSource",
	EventTime:          time.Now(),
}

type roundTripFunc func(r *http.Request) *http.Response

func (s roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r), nil
}

func Test_EventForwarder_BadCtx(t *testing.T) {
	fwd := NewEventForwarder("testtarget")

	brokenCtx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventTime:          time.Now(),
	}
	err := fwd.PostEvent(brokenCtx, []byte{})

	if err == nil {
		t.Errorf("Expected an error but got none.")
	}
}

func Test_EventForwarder_TerminalHttpFailure(t *testing.T) {
	fwd := NewEventForwarder("testtarget")
	err := fwd.PostEvent(testCtx, []byte{})

	if err == nil {
		t.Errorf("Expected an error but got none.")
	}
}

func Test_EventForwarder_SoftHttpFailure(t *testing.T) {
	fwd := NewEventForwarder("testtarget")
	fwd.httpClient.Transport = roundTripFunc(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: 500,
			Body:       ioutil.NopCloser(bytes.NewBufferString("Nope")),
			Header:     make(http.Header),
		}
	})

	err := fwd.PostEvent(testCtx, []byte{})

	if err == nil {
		t.Errorf("Expected an error but got none.")
	}
}

func Test_EventForwarder_AllGood(t *testing.T) {
	fwd := NewEventForwarder("testtarget")
	fwd.httpClient.Transport = roundTripFunc(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewBufferString("OK")),
			Header:     make(http.Header),
		}
	})

	err := fwd.PostEvent(testCtx, []byte{})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
