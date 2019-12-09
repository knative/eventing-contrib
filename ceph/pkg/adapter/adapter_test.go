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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ceph "knative.dev/eventing-contrib/ceph/pkg/apis/v1alpha1"
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
	port := "28080"
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
	go func() {
		sinkServer = httptest.NewServer(&fakeSink{
			t:            t,
			expectedBody: string(notification1Buffer),
		},
		)
		Start(sinkServer.URL, port)
	}()
	time.Sleep(5 * time.Second)
	result, err := http.Post("http://:"+port+"/", "application/json", bytes.NewBuffer(jsonBuffer))
	if err != nil {
		t.Error(err)
	}
	defer result.Body.Close()
	_, err = ioutil.ReadAll(result.Body)
	if err != nil {
		t.Error(err)
	}
	Stop()
}

type fakeSink struct {
	t              *testing.T
	expectedBody   string
	expectedHeader string
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
