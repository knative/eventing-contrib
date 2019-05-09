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

package adapter

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go/service/sqs"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	timestamp := "1542107977907705474"
	testCases := map[string]struct {
		sink    func(http.ResponseWriter, *http.Request)
		reqBody string
		error   bool
	}{
		"happy": {
			sink:    sinkAccepted,
			reqBody: `{"Attributes":{"SentTimestamp":"1542107977907705474"},"Body":"The body","MD5OfBody":null,"MD5OfMessageAttributes":null,"MessageAttributes":null,"MessageId":"ABC01","ReceiptHandle":null}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"Attributes":{"SentTimestamp":"1542107977907705474"},"Body":"The body","MD5OfBody":null,"MD5OfMessageAttributes":null,"MessageAttributes":null,"MessageId":"ABC01","ReceiptHandle":null}`,
			error:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				QueueURL:             "https://sqs.us-east-2.amazonaws.com/123123/MyQueue",
				SinkURI:              sinkServer.URL,
				OnFailedPollWaitSecs: 1,
			}

			if err := a.initClient(); err != nil {
				t.Errorf("failed to create cloudevent client, %v", err)
			}

			body := "The body"
			messageID := "ABC01"
			attrs := map[string]*string{
				"SentTimestamp": &timestamp,
			}
			m := &sqs.Message{
				MessageId:  &messageID,
				Body:       &body,
				Attributes: attrs,
			}
			err := a.postMessage(context.TODO(), zap.S(), m)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
		})
	}
}

func TestGetRegionInvalidUrl(t *testing.T) {
	actual, err := getRegion("sdfadf")
	if err == nil {
		t.Error("Expecting an error but got a result", actual)
	}
}

func TestGetRegionValidUrl(t *testing.T) {
	expected := "eu-west-1"
	actual, _ := getRegion("https://sqs.eu-west-1.amazonaws.com/23234/test")
	if expected != actual {
		t.Error("Expecting", expected, "but got", actual)
	}
}

func TestReceiveMessage_ServeHTTP(t *testing.T) {

	id := "ABC01"
	body := "the body"
	timestamp := "1542107977907705474"
	m := &sqs.Message{
		MessageId:  &id,
		Body:       &body,
		Attributes: map[string]*string{"SentTimestamp": &timestamp},
	}

	testCases := map[string]struct {
		sink  func(http.ResponseWriter, *http.Request)
		acked bool
	}{
		"happy": {
			sink:  sinkAccepted,
			acked: true,
		},
		"rejected": {
			sink:  sinkRejected,
			acked: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				QueueURL: "https://sqs.us-east-2.amazonaws.com/123123/MyQueue",
				SinkURI:  sinkServer.URL,
			}

			if err := a.initClient(); err != nil {
				t.Errorf("failed to create cloudevent client, %v", err)
			}

			ack := new(bool)

			a.receiveMessage(context.TODO(), m, func() { *ack = true })

			if tc.acked != *ack {
				t.Error("expected message ack ", tc.acked, " but real is ", *ack)
			}

		})
	}
}

func TestPollLoopCancels(t *testing.T) {

	a := &Adapter{
		QueueURL: "https://test.sqs.aws/123123",
		SinkURI:  "https://sink.server/",
	}

	ctx := context.Background()
	stopCh := make(chan struct{}, 1)

	msg := struct{}{}
	stopCh <- msg
	err := a.pollLoop(ctx, nil, stopCh)
	if err != nil {
		t.Error("expected pollLoop to return cleanly, but got", err)
	}
}

func TestStart(t *testing.T) {

	a := &Adapter{
		QueueURL: "https://test.sqs.aws/123123",
		SinkURI:  "https://sink.server/",
	}

	ctx := context.Background()
	stopCh := make(chan struct{}, 1)

	msg := struct{}{}
	stopCh <- msg
	err := a.Start(ctx, stopCh)
	if err != nil {
		t.Error("expected Start to return cleanly, but got", err)
	}
}

type fakeHandler struct {
	body []byte

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}
