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
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
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
				EventSource:     "test-source",
				ServerURL:       "http://server.url",
				PromQL:          "prom-ql",
				AuthTokenFile:   "auth_token_file",
				CACertConfigMap: "ca_cert_config_map",
				Schedule:        "* * * * *",
				Step:            "30s",
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
			if diff := cmp.Diff(tc.opt.AuthTokenFile, got.authTokenFile); diff != "" {
				t.Errorf("unexpected authTokenFile diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.CACertConfigMap, got.caCertConfigMap); diff != "" {
				t.Errorf("unexpected caCertConfigMap diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.Schedule, got.schedule); diff != "" {
				t.Errorf("unexpected schedule diff (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.opt.Step, got.step); diff != "" {
				t.Errorf("unexpected step diff (-want, +got) = %v", diff)
			}
		})
	}
}

func TestStartAdaptor(t *testing.T) {
	ce := kncetesting.NewTestClient()

	testCases := map[string]struct {
		opt        envConfig
		wantErrMsg string
	}{
		"bad-schedule": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				ServerURL:   "http://server.url",
				PromQL:      "prom-ql",
				Schedule:    "bad_schedule",
			},
			wantErrMsg: "expected exactly 5 fields, found 1: [bad_schedule]",
		},
		"no-schedule": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				ServerURL:   "http://server.url",
				PromQL:      "prom-ql",
			},
			wantErrMsg: "empty spec string",
		},
		"bad-auth-token-file": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource:   "test-source",
				ServerURL:     "http://server.url",
				PromQL:        "prom-ql",
				Schedule:      "* * * * *",
				AuthTokenFile: "no_such_file",
			},
			wantErrMsg: "open no_such_file: no such file or directory",
		},
		"bad-ca-cert-config-map": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource:     "test-source",
				ServerURL:       "http://server.url",
				PromQL:          "prom-ql",
				Schedule:        "* * * * *",
				CACertConfigMap: "no_such_config_map",
			},
			wantErrMsg: "open /etc/no_such_config_map/service-ca.crt: no such file or directory",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			r := &mockReporter{}
			ctx, _ := pkgtesting.SetupFakeContext(t)
			logger := zap.NewExample().Sugar()
			ctx = logging.WithLogger(ctx, logger)

			a := NewAdapter(ctx, &tc.opt, ce, r)
			stopCh := make(chan struct{})
			err := a.Start(stopCh)

			if diff := cmp.Diff(tc.wantErrMsg, err.Error()); diff != "" {
				t.Errorf("unexpected err diff (-want, +got) = %v", diff)
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
	const promQL = `promQL`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "{\"request_uri\":\""+r.RequestURI+"\"}")
	}))
	defer ts.Close()

	testCases := map[string]struct {
		opt            envConfig
		wantRequestURI string
	}{
		"instant-query": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				ServerURL:   ts.URL,
				PromQL:      promQL,
				Schedule:    "* * * * *",
			},
			wantRequestURI: "/api/v1/query?query=" + promQL,
		},
		"range-query": {
			opt: envConfig{
				EnvConfig: adapter.EnvConfig{
					Namespace: "test-ns",
				},
				EventSource: "test-source",
				ServerURL:   ts.URL,
				PromQL:      promQL,
				Schedule:    "* * * * *",
				Step:        "5s",
			},
			wantRequestURI: "/api/v1/query_range?query=" + promQL + "&start=",
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ce := newAdapterTestClient()

			r := &mockReporter{}
			ctx, _ := pkgtesting.SetupFakeContext(t)
			logger := zap.NewExample().Sugar()
			ctx = logging.WithLogger(ctx, logger)

			a := NewAdapter(ctx, &tc.opt, ce, r)

			done := make(chan struct{})
			go func() {
				a.Start(ce.stopCh)
				done <- struct{}{}
			}()
			<-done

			validateSent(t, ce, tc.wantRequestURI)
		})
	}
}

func validateSent(t *testing.T, ce *adapterTestClient, wantData string) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	if got := ce.Sent()[0].Data; !strings.Contains(string(got.([]byte)), wantData) {
		t.Errorf("Expected %q event to be sent, got %q", wantData, string(got.([]byte)))
	}
}
