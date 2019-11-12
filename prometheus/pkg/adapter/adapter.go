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

	cloudevents "github.com/cloudevents/sdk-go"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	"knative.dev/eventing-contrib/prometheus/pkg/apis/sources/v1alpha1"
)

const (
	apiChunkOfURL = `/api/v1/query?query=`
)

type envConfig struct {
	adapter.EnvConfig

	EventSource     string `envconfig:"EVENT_SOURCE" required:"true"`
	ServerURL       string `envconfig:"PROMETHEUS_SERVER_URL" required:"true"`
	PromQL          string `envconfig:"PROMETHEUS_PROM_QL" required:"true"`
	AuthTokenFile   string `envconfig:"PROMETHEUS_AUTH_TOKEN_FILE" required:"false"`
	CACertConfigMap string `envconfig:"PROMETHEUS_CA_CERT_CONFIG_MAP" required:"false"`
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
	caCertConfigMap string
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
	}

	return a
}

func (a *prometheusAdapter) Start(stopCh <-chan struct{}) error {
	completeURL := a.serverURL + apiChunkOfURL + a.promQL

	var err error
	a.req, err = http.NewRequest(`GET`, completeURL, nil)
	if err != nil {
		a.logger.Error("HTTP request error", zap.Error(err))
		return err
	}
	if a.authTokenFile != "" {
		content, err := ioutil.ReadFile(a.authTokenFile)
		if err != nil {
			a.logger.Error("Error reading authentication token from "+a.authTokenFile, zap.Error(err))
			return err
		}
		a.req.Header.Set("Authorization", "Bearer "+string(content))
	}

	a.logger.Info(a.req)

	a.client = &http.Client{}

	if a.caCertConfigMap != "" {
		caCertFile := "/etc/" + a.caCertConfigMap + "/service-ca.crt"
		caCert, err := ioutil.ReadFile(caCertFile)
		if err != nil {
			a.logger.Error("Error reading CA certificate from "+caCertFile, zap.Error(err))
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
	wait.Until(a.send, 5*time.Second, stopCh)
	return nil
}

func (a *prometheusAdapter) send() {
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

		if _, _, err := a.ce.Send(context.Background(), *event); err != nil {
			a.logger.Error("Cloud Event delivery error", zap.Error(err))
			return
		}
	}
}

func (a *prometheusAdapter) makeEvent(payload interface{}) (*cloudevents.Event, error) {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
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
