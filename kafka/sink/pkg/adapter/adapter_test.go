/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
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
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/adapter/v2"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

const (
	eventID = "12345"
)

// testCase holds a single row of our GitHubSource table tests
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run()
	name string

	// payload contains the CloudEvents event payload
	payload interface{}
}

// type TestPayload struct {
// 	Id string `json:"id"`
// }

var testCases = []testCase{
	// 	{
	// 		name: "valid cloud event",
	// 		payload: func() interface{} {
	// 			pl := TestPayload{}
	// 			pl.Id = eventID
	// 			retun pl
	// 		}(),
	// 	},
}

func newTestAdapter(t *testing.T, ce cloudevents.Client) *kafkaSinkAdapter {
	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "default",
		},
		Port:        "8023",
		KafkaServer: "test.kafka",
		Topic:       "test-topic",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)
	logger := zap.NewExample().Sugar()
	ctx = logging.WithLogger(ctx, logger)

	return NewAdapter(ctx, &env, ce).(*kafkaSinkAdapter)
}

func TestGracefulShutdown(t *testing.T) {
	// ce := adaptertest.NewTestClient()
	// ra := newTestAdapter(t, ce)
	// ctx, cancel := context.WithCancel(context.Background())

	// go func(cancel context.CancelFunc) {
	// 	defer cancel()
	// 	time.Sleep(time.Second)

	// }(cancel)

	// 	t.Logf("starting Kafka Sink server")

	// 	err := ra.Start(ctx)
	// 	if err != nil {
	// 		t.Error(err)
	// 	}
}

func TestServer(t *testing.T) {
	// for _, tc := range testCases {
	// 	ce := adaptertest.NewTestClient()
	// 	adapter := newTestAdapter(t, ce)

	// 	//router := adapter.newRouter()
	// 	//server := httptest.NewServer(router)
	// 	//defer server.Close()

	// 	//t.Run(tc.name, tc.runner(t, server.URL, ce))
	// }
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, url string, ceClient *adaptertest.TestCloudEventsClient) func(t *testing.T) {
	return func(t *testing.T) {
		// if tc.eventType == "" {
		// 	t.Fatal("eventType is required for table tests")
		// }
		body, _ := json.Marshal(tc.payload)
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		//req.Header.Set(common.GHHeaderEvent, tc.eventType)
		//req.Header.Set(common.GHHeaderDelivery, eventID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		tc.validateAcceptedPayload(t, ceClient)
	}
}

func (tc *testCase) validateAcceptedPayload(t *testing.T, ce *adaptertest.TestCloudEventsClient) {
	t.Helper()
	if len(ce.Sent()) != 1 {
		return
	}
}
