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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter"
	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/source"
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
	}{
		"test": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				ServerURL:   "http://server.url",
				PromQL:      "prom-ql",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			r := &mockReporter{}
			ctx, _ := pkgtesting.SetupFakeContext(t)
			logger := zap.NewExample().Sugar()
			ctx = logging.WithLogger(ctx, logger)

			a := NewAdapter(ctx, &tc.opt, ce, r)

			got, ok := a.(*prometheusAdapter)
			if !ok {
				t.Errorf("expected NewAdapter to return a *prometheusAdapter, but did not")
			}
			if logger != got.logger {
				t.Errorf("unexpected logger diff, want: %p, got %p", logger, got.logger)
			}
			if diff := cmp.Diff(tc.opt.EnvConfig.Namespace, got.namespace); diff != "" {
				t.Errorf("unexpected namespace diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.EventSource, got.source); diff != "" {
				t.Errorf("unexpected source diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.ServerURL, got.serverURL); diff != "" {
				t.Errorf("unexpected serverURL diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.PromQL, got.promQL); diff != "" {
				t.Errorf("unexpected promQL diff (-want, +got) = %v", diff)
			}
		})
	}
}

type adapterTestClient struct {
	*kncetesting.TestCloudEventsClient
	stopCh chan struct{}
}

var _ cloudevents.Client = (*adapterTestClient)(nil)

func newAdapterTestClient() *adapterTestClient {
	return &adapterTestClient{
		kncetesting.NewTestClient(),
		make(chan struct{}, 1),
	}
}

func (c *adapterTestClient) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	retCtx, retEvent, retError := c.TestCloudEventsClient.Send(ctx, event)
	c.stopCh <- struct{}{}
	return retCtx, retEvent, retError
}

func TestReceiveEventPoll(t *testing.T) {
	const promReply = "{\"foo\":\"bar\"}"
	const promQL = `promQL`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantRequestURI := apiChunkOfURL + promQL
		if diff := cmp.Diff(wantRequestURI, r.RequestURI); diff != `` {
			t.Errorf(`unexpected promQL diff (-want, +got) = %v`, diff)
		}
		fmt.Fprintln(w, promReply)
	}))
	defer ts.Close()

	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "test-ns",
		},
		EventSource: "test-source",
		ServerURL:   ts.URL,
		PromQL:      promQL,
	}

	ce := newAdapterTestClient()

	r := &mockReporter{}
	ctx, _ := pkgtesting.SetupFakeContext(t)
	logger := zap.NewExample().Sugar()
	ctx = logging.WithLogger(ctx, logger)

	a := NewAdapter(ctx, &env, ce, r)

	done := make(chan struct{})
	go func() {
		a.Start(ce.stopCh)
		done <- struct{}{}
	}()
	<-done

	validateSent(t, ce, promReply+"\n")
}

func validateSent(t *testing.T, ce *adapterTestClient, wantData string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data; string(got.([]byte)) != wantData {
		t.Errorf("Expected %q event to be sent, got %q", wantData, string(got.([]byte)))
	}
}
