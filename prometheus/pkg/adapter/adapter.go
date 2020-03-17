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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"github.com/robfig/cron"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	"knative.dev/eventing-contrib/prometheus/pkg/apis/sources/v1alpha1"
)

type envConfig struct {
	adapter.EnvConfig

	EventSource     string `envconfig:"EVENT_SOURCE" required:"true"`
	ServerURL       string `envconfig:"PROMETHEUS_SERVER_URL" required:"true"`
	PromQL          string `envconfig:"PROMETHEUS_PROM_QL" required:"true"`
	AuthTokenFile   string `envconfig:"PROMETHEUS_AUTH_TOKEN_FILE" required:"false"`
	CACertConfigMap string `envconfig:"PROMETHEUS_CA_CERT_CONFIG_MAP" required:"false"`
	Schedule        string `envconfig:"PROMETHEUS_SCHEDULE" required:"true"`
	Step            string `envconfig:"PROMETHEUS_STEP" required:"false"`
}

type prometheusAdapter struct {
	source          string
	ce              cloudevents.Client
	reporter        source.StatsReporter
	namespace       string
	logger          *zap.SugaredLogger
	serverURL       string
	promQL          string
	authTokenFile   string
	authToken       string
	caCertConfigMap string
	schedule        string
	step            string
	lastRun         time.Time
	req             *http.Request
	client          *http.Client
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// NewAdapter creates an adapter to convert PromQL replies to CloudEvents
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	a := &prometheusAdapter{
		source:          env.EventSource,
		ce:              ceClient,
		reporter:        reporter,
		logger:          logger,
		namespace:       env.Namespace,
		serverURL:       env.ServerURL,
		promQL:          env.PromQL,
		authTokenFile:   env.AuthTokenFile,
		caCertConfigMap: env.CACertConfigMap,
		schedule:        env.Schedule,
		step:            env.Step,
		lastRun:         time.Now(),
	}

	return a
}

func (a *prometheusAdapter) Start(stopCh <-chan struct{}) error {
	if err := a.readAuthTokenIfNeeded(); err != nil {
		return err
	}
	// pre-make an immutable HTTP Request for an instant query
	if a.step == "" {
		if err := a.makeHTTPRequest(); err != nil {
			return err
		}
	}
	if err := a.makeHTTPClient(); err != nil {
		return err
	}

	sched, err := cron.ParseStandard(a.schedule)
	if err != nil {
		a.logger.Errorf("Unparseable schedule %s: %v", a.schedule, err)
		return err
	}

	c := cron.New()
	c.Schedule(sched, cron.FuncJob(a.send))
	c.Start()
	<-stopCh
	c.Stop()
	return nil
}

func (a *prometheusAdapter) send() {
	// range query
	if a.step != "" {
		if err := a.makeHTTPRequest(); err != nil {
			return
		}
		a.lastRun = time.Now()
	}
	resp, err := a.client.Do(a.req)
	if err != nil {
		a.logger.Error("HTTP invocation error", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	reply, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("HTTP reply error", zap.Error(err))
		return
	}
	if len(reply) > 0 {
		event, err := a.makeEvent(reply)
		if err != nil {
			a.logger.Error("Cloud Event creation error", zap.Error(err))
			return
		}
		if _, _, err = a.ce.Send(context.Background(), *event); err != nil {
			a.logger.Error("Cloud Event delivery error", zap.Error(err))
			return
		}
	}
}

func (a *prometheusAdapter) makeEvent(payload interface{}) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetSource(a.source)
	event.SetID(string(uuid.NewUUID()))
	event.SetType(v1alpha1.PromQLPrometheusSourceEventType)
	event.SetDataContentType(cloudevents.ApplicationJSON)

	if err := event.SetData(payload); err != nil {
		return nil, err
	}

	a.logger.Info(&event)

	return &event, nil
}

func (a *prometheusAdapter) makeInvocationURL() string {
	range_query := (a.step != "")
	ret := a.serverURL + `/api/v1/query`
	if range_query {
		ret += `_range`
	}
	ret += `?query=` + a.promQL
	if range_query {
		ret += `&start=` + a.lastRun.Format(time.RFC3339) +
			`&end=` + time.Now().Format(time.RFC3339) +
			`&step=` + a.step
	}
	return ret
}

func (a *prometheusAdapter) makeHTTPRequest() error {
	var err error
	if a.req, err = http.NewRequest(`GET`, a.makeInvocationURL(), nil); err != nil {
		a.logger.Error("HTTP request error", zap.Error(err))
		return err
	}
	if a.authToken != "" {
		a.req.Header.Set("Authorization", "Bearer "+a.authToken)
	}
	a.logger.Info(a.req)
	return nil
}

func (a *prometheusAdapter) makeHTTPClient() error {
	a.client = &http.Client{}

	if a.caCertConfigMap != "" {
		caCertFile := "/etc/" + a.caCertConfigMap + "/service-ca.crt"
		caCert, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			a.logger.Error("Error reading CA certificate from "+caCertFile+": ", zap.Error(err))
			return err
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			a.logger.Error("Error parsing CA certificate from " + caCertFile)
			return err
		}
		a.client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}
	return nil
}

func (a *prometheusAdapter) readAuthTokenIfNeeded() error {
	if a.authTokenFile != "" {
		content, err := ioutil.ReadFile(a.authTokenFile)
		if err != nil {
			a.logger.Error("Error reading authentication token from "+a.authTokenFile+": ", zap.Error(err))
			return err
		}
		a.authToken = string(content)
	}
	return nil
}
