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
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	ceph "knative.dev/eventing-contrib/ceph/pkg/apis/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
)

var notification1 = ceph.BucketNotification{
	EventVersion: "2.1",
	EventSource:  "ceph:s3",
	AwsRegion:    "tenantA",
	EventTime:    "2019-11-22T13:47:35.124724Z",
	EventName:    "s3:ObjectCreated:Put",
	UserIdentity: ceph.UserIdentitySpec{
		PrincipalID: "tester",
	},
	RequestParameters: ceph.RequestParametersSpec{
		SourceIPAddress: "",
	},
	ResponseElements: ceph.ResponseElementsSpec{
		XAmzRequestID: "503a4c37-85eb-47cd-8681-2817e80b4281.5330.903595",
		XAmzID2:       "14d2-a1-a",
	},
	S3: ceph.S3Spec{
		S3SchemaVersion: "1.0",
		ConfigurationID: "fishbucket_notifications",
		Bucket: ceph.BucketSpec{
			Name: "fishbucket",
			OwnerIdentity: ceph.OwnerIdentitySpec{
				PrincipalID: "tester",
			},
			Arn: "arn:aws:s3:::fishbucket",
			ID:  "503a4c37-85eb-47cd-8681-2817e80b4281.5332.38",
		},
		Object: ceph.ObjectSpec{
			Key:       "fish9.jpg",
			Size:      1024,
			ETag:      "37b51d194a7513e45b56f6524f2d51f2",
			VersionID: "",
			Sequencer: "F7E6D75DC742D108",
			Metadata: []ceph.MetadataEntry{
				{Key: "x-amz-meta-meta1", Value: "This is my metadata value"},
				{Key: "x-amz-meta-meta2", Value: "This is another metadata value"},
			},
		},
	},
	EventID: "1575221657.102001.80dad3aad8584778352c68ab06250327",
}

var jsonData = ceph.BucketNotifications{
	Records: []ceph.BucketNotification{
		notification1,
	},
}

func TestRecords(t *testing.T) {
	var sinkServer *httptest.Server
	defer func() {
		if sinkServer != nil {
			sinkServer.Close()
		}
	}()
	jsonBuffer, err := json.Marshal(jsonData)
	if err != nil {
		t.Error(err)
	}
	notification1Buffer, err := json.Marshal(notification1)
	if err != nil {
		t.Error(err)
	}

	var ca *cephReceiveAdapter
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sinkServer = httptest.NewServer(&fakeSink{
			t:            t,
			expectedBody: string(notification1Buffer),
		},
		)
		mu.Lock()
		ca = newTestAdapter(t, adaptertest.NewTestClient(), sinkServer.URL)
		mu.Unlock()
		err = ca.Start(ctx)
		if err != nil {
			t.Error(err)
		}
	}()
	time.Sleep(5 * time.Second)

	mu.Lock()
	sinkPort := ca.port
	mu.Unlock()

	result, err := http.Post("http://:"+sinkPort+"/", "application/json", bytes.NewBuffer(jsonBuffer))
	if err != nil {
		t.Error(err)
	}
	defer result.Body.Close()
	_, err = ioutil.ReadAll(result.Body)
	if err != nil {
		t.Error(err)
	}
	cancel()
}

type fakeSink struct {
	t            *testing.T
	expectedBody string
}

func (h *fakeSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.t.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if string(body) != h.expectedBody {
		h.t.Error("Notification Body Mismatch")
	}
	w.WriteHeader(http.StatusOK)
}

func newTestAdapter(t *testing.T, ce cloudevents.Client, sinkServerURL string) *cephReceiveAdapter {
	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "default",
		},
		Port: "28080",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)
	logger := zap.NewExample().Sugar()
	ctx = logging.WithLogger(ctx, logger)
	ctx = cloudevents.ContextWithTarget(ctx, sinkServerURL)

	return NewAdapter(ctx, &env, ce).(*cephReceiveAdapter)
}
