/*
Copyright 2020 The Knative Authors

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

package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	protocolhttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
)

// This dispatcher tests the full integration of the dispatcher code with Kafka.
// This test doesn't run on the CI because unit tests script doesn't start a Kafka cluster.
// Use it in emergency situations when you can't reproduce the e2e test failures and the failure might be
// in the dispatcher code.
// Start a kafka cluster with docker: docker run --rm --net=host -e ADV_HOST=localhost -e SAMPLEDATA=0 lensesio/fast-data-dev
// Keep also the port 8080 free for the MessageReceiver
func TestDispatcher(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skipf("This test can't run in CI")
	}

	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.WarnLevel))
	if err != nil {
		t.Fatal(err)
	}

	tracing.SetupStaticPublishing(logger.Sugar(), "localhost", tracing.AlwaysSample)

	dispatcherArgs := KafkaDispatcherArgs{
		KnCEConnectionArgs: nil,
		ClientID:           "testing",
		Brokers:            []string{"localhost:9092"},
		TopicFunc:          utils.TopicName,
		Logger:             logger,
	}

	// Create the dispatcher. At this point, if Kafka is not up, this thing fails
	dispatcher, err := NewDispatcher(context.Background(), &dispatcherArgs)
	if err != nil {
		t.Skipf("no dispatcher: %v", err)
	}

	// Start the dispatcher
	go func() {
		if err := dispatcher.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(1 * time.Second)

	// We need a channelaproxy and channelbproxy for handling correctly the Host header
	channelAProxy := httptest.NewServer(createReverseProxy(t, "channela.svc"))
	defer channelAProxy.Close()
	channelBProxy := httptest.NewServer(createReverseProxy(t, "channelb.svc"))
	defer channelBProxy.Close()

	// Start a bunch of test servers to simulate the various services
	transformationsWg := sync.WaitGroup{}
	transformationsWg.Add(1)
	transformationsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer transformationsWg.Done()
		message := protocolhttp.NewMessageFromHttpRequest(r)
		defer message.Finish(nil)

		err := protocolhttp.WriteResponseWriter(context.Background(), message, 200, w, transformer.AddExtension("transformed", "true"))
		if err != nil {
			w.WriteHeader(500)
			t.Fatal(err)
		}
	}))
	defer transformationsServer.Close()

	receiverWg := sync.WaitGroup{}
	receiverWg.Add(1)
	receiverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer receiverWg.Done()
		transformed := r.Header.Get("ce-transformed")
		if transformed != "true" {
			w.WriteHeader(500)
			t.Fatalf("Expecting ce-transformed: true, found %s", transformed)
		}
	}))
	defer receiverServer.Close()

	transformationsFailureWg := sync.WaitGroup{}
	transformationsFailureWg.Add(1)
	transformationsFailureServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer transformationsFailureWg.Done()
		w.WriteHeader(500)
	}))
	defer transformationsFailureServer.Close()

	deadLetterWg := sync.WaitGroup{}
	deadLetterWg.Add(1)
	deadLetterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer deadLetterWg.Done()
		transformed := r.Header.Get("ce-transformed")
		if transformed != "" {
			w.WriteHeader(500)
			t.Fatalf("Not expecting ce-transformed, found %s", transformed)
		}
	}))
	defer deadLetterServer.Close()

	logger.Debug("Test servers",
		zap.String("transformations server", transformationsServer.URL),
		zap.String("transformations failure server", transformationsFailureServer.URL),
		zap.String("receiver server", receiverServer.URL),
		zap.String("dead letter server", deadLetterServer.URL),
	)

	// send -> channela -> sub with transformationServer and reply to channelb -> channelb -> sub with receiver -> receiver
	config := multichannelfanout.Config{
		ChannelConfigs: []multichannelfanout.ChannelConfig{
			{
				Namespace: "default",
				Name:      "channela",
				HostName:  "channela.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []eventingduck.SubscriberSpec{{
						UID:           "aaaa",
						Generation:    1,
						SubscriberURI: mustParseUrl(t, transformationsServer.URL),
						ReplyURI:      mustParseUrl(t, channelBProxy.URL),
					}, {
						UID:           "cccc",
						Generation:    1,
						SubscriberURI: mustParseUrl(t, transformationsFailureServer.URL),
						ReplyURI:      mustParseUrl(t, channelBProxy.URL),
						Delivery: &eventingduck.DeliverySpec{
							DeadLetterSink: &duckv1.Destination{URI: mustParseUrl(t, deadLetterServer.URL)},
						},
					}},
				},
			},
			{
				Namespace: "default",
				Name:      "channelb",
				HostName:  "channelb.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []eventingduck.SubscriberSpec{{
						UID:           "bbbb",
						Generation:    1,
						SubscriberURI: mustParseUrl(t, receiverServer.URL),
					}},
				},
			},
		},
	}

	err = dispatcher.UpdateHostToChannelMap(&config)
	if err != nil {
		t.Fatal(err)
	}

	failed, err := dispatcher.UpdateKafkaConsumers(&config)
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 0 {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	// Ok now everything should be ready to send the event
	httpsender, err := kncloudevents.NewHttpMessageSender(nil, channelAProxy.URL)
	if err != nil {
		t.Fatal(err)
	}

	req, err := httpsender.NewCloudEventRequest(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	event := test.FullEvent()
	_ = protocolhttp.WriteRequest(context.Background(), binding.ToMessage(&event), req)

	res, err := httpsender.Send(req)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != 202 {
		t.Fatalf("Expected 202, Have %d", res.StatusCode)
	}

	transformationsFailureWg.Wait()
	deadLetterWg.Wait()
	transformationsWg.Wait()
	receiverWg.Wait()
}

func createReverseProxy(t *testing.T, host string) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		target := mustParseUrl(t, "http://localhost:8080")
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
		req.Host = host
	}
	return &httputil.ReverseProxy{Director: director}
}

func mustParseUrl(t *testing.T, str string) *apis.URL {
	url, err := apis.ParseURL(str)
	if err != nil {
		t.Fatal(err)
	}
	return url
}
