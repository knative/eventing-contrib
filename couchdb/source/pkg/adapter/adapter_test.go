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

package adapter

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/v2"
	kncetesting "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	"github.com/go-kivik/kivik/v3/driver"
	"github.com/go-kivik/kivikmock/v3"
)

func TestNewAdapter(t *testing.T) {
	ce := kncetesting.NewTestClient()

	testCases := map[string]struct {
		opt envConfig

		wantNamespace string
		wantDatabase  string
	}{
		"with source": {
			opt: envConfig{
				EventSource: "test-source",
				Database:    "mydb",
			},
			wantDatabase: "mydb",
		},
		"with namespace": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				Database:    "mydb",
			},
			wantNamespace: "test-ns",
			wantDatabase:  "mydb",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx, _ := pkgtesting.SetupFakeContext(t)
			c, mock := kivikmock.NewT(t)

			mock.ExpectDB()

			a := newAdapter(ctx, &tc.opt, ce, c.DSN(), "kivikmock")

			got, ok := a.(*couchDbAdapter)
			if !ok {
				t.Errorf("expected NewAdapter to return a *couchDbAdapter, but did not")
			}
			if diff := cmp.Diff(tc.opt.EventSource, got.source); diff != "" {
				t.Errorf("unexpected source diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.wantNamespace, got.namespace); diff != "" {
				t.Errorf("unexpected namespace diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.wantDatabase, got.couchDB.Name()); diff != "" {
				t.Errorf("unexpected namespace diff (-want, +got) = %v", diff)
			}
		})
	}
}

type adapterTestClient struct {
	*kncetesting.TestCloudEventsClient
	cancel context.CancelFunc
}

var _ cloudevents.Client = (*adapterTestClient)(nil)

func newAdapterTestClient(cancel context.CancelFunc) *adapterTestClient {
	return &adapterTestClient{
		kncetesting.NewTestClient(),
		cancel,
	}
}

func (c *adapterTestClient) Send(ctx context.Context, event cloudevents.Event) cloudevents.Result {
	retError := c.TestCloudEventsClient.Send(ctx, event)
	c.cancel()
	return retError
}

func TestReceiveEventPoll(t *testing.T) {
	testCases := map[string]struct {
		feed string
	}{
		"normal feed": {
			feed: "normal",
		},
		"continuous feed": {
			feed: "continuous",
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			env := envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "default",
				},
				EventSource: "test-source",
				Database:    "testdb",
				Feed:        tc.feed,
			}
			ctx, _ := pkgtesting.SetupFakeContext(t)
			logger := zap.NewExample().Sugar()
			ctx = logging.WithLogger(ctx, logger)

			c, mock := kivikmock.NewT(t)

			mockDB := mock.NewDB()
			mock.ExpectDB().WithName("testdb").WillReturn(mockDB)
			mockDB.ExpectChanges().WillReturn(kivikmock.NewChanges().AddChange(&driver.Change{
				ID:      "anid",
				Seq:     "aseq",
				Changes: driver.ChangedRevs{"arev"},
			}))

			ctx, cancel := context.WithCancel(context.Background())
			ce := newAdapterTestClient(cancel)

			a := newAdapter(ctx, &env, ce, c.DSN(), "kivikmock").(*couchDbAdapter)

			done := make(chan struct{})
			go func() {
				err := a.Start(ctx)
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				done <- struct{}{}
			}()
			<-done

			validateSent(t, ce, `["arev"]`)
		})
	}
}

func validateSent(t *testing.T, ce *adapterTestClient, wantData string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data(); string(got) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, string(got))
	}
}
