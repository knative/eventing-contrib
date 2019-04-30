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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/google/go-cmp/cmp"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	sourceName = "test-apiserver-adapter"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
)

type testCase struct {
	controllertesting.TestCase

	// Where to send events
	sink func(http.ResponseWriter, *http.Request)

	// Expected event data
	data interface{}
}

func TestReconcile(t *testing.T) {
	testCases := []testCase{
		{
			TestCase: controllertesting.TestCase{
				Name: "Receive Pod creation event",
				InitialState: []runtime.Object{
					getPod(),
				},
				Reconciles: getPod(),
			},
			sink: sinkAccepted,
			data: decode(t, encode(t, getPodRef())),
		},
	}

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: testNS,
			Name:      sourceName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Create fake sink server
			h := &fakeHandler{
				handler: tc.sink,
			}

			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			// Bind cloud event client
			ceClient, err := kncloudevents.NewDefaultClient(sinkServer.URL)
			if err != nil {
				t.Errorf("cannot create cloud event client: %v", zap.Error(err))
			}

			// Create fake k8s client
			c := fake.NewFakeClient(tc.InitialState...)

			r := &reconciler{
				client:       c,
				eventsClient: ceClient,
				gvk:          tc.Reconciles.GetObjectKind().GroupVersionKind(),
			}

			_, err = r.Reconcile(request)
			if err != nil {
				t.Errorf("Expected no error")
			}

			if diff := cmp.Diff(tc.data, decode(t, h.body)); diff != "" {
				t.Errorf("incorrect event (-want, +got): %v", diff)
			}
		})
	}
}

func getPod() runtime.Object {
	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceName,
			Namespace: testNS,
			SelfLink:  "/apis/v1/namespaces/" + testNS + "/pod/" + sourceName,
		},
	}
}

func getPodRef() corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Pod",
		Name:       sourceName,
		Namespace:  testNS,
	}
}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body
	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func encode(t *testing.T, data interface{}) string {
	b, err := json.Encode(data)
	if err != nil {
		t.Fatalf("failed to encode data: %v", err)
	}
	return string(b)
}

func decode(t *testing.T, data interface{}) interface{} {
	var out interface{}
	err := json.Decode(data, &out)
	if err != nil {
		t.Fatalf("failed to decode data: %v", err)
	}
	return out
}
