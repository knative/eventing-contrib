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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter"
	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/source"

	"github.com/go-kivik/kivik/driver"
	"github.com/go-kivik/kivikmock"
)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount += 1
	return nil
}

func TestNewAdaptor(t *testing.T) {
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
			r := &mockReporter{}
			ctx, _ := pkgtesting.SetupFakeContext(t)
			c, mock := kivikmock.NewT(t)

			mock.ExpectDB()

			a := newAdapter(ctx, &tc.opt, ce, r, c.DSN(), "kivikmock")

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

func TestReceiveEventPoll(t *testing.T) {
	ce := kncetesting.NewTestClient()

	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "default",
		},
		EventSource: "test-source",
		Database:    "testdb",
	}
	r := &mockReporter{}
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

	a := newAdapter(ctx, &env, ce, r, c.DSN(), "kivikmock").(*couchDbAdapter)

	stopCh := make(chan struct{})
	done := make(chan struct{})
	stopped := false
	go func() {
		a.Start(stopCh)
		stopped = true
		done <- struct{}{}
	}()

	time.Sleep(3 * time.Second) // Run at least onece

	validateSent(t, ce, `["arev"]`)

	if !stopped {
		stopCh <- struct{}{}
	}
	<-done
}

func validateSent(t *testing.T, ce *kncetesting.TestCloudEventsClient, wantData string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data; string(got.([]byte)) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, string(got.([]byte)))
	}
}
