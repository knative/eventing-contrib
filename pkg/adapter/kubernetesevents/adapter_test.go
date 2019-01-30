/*
Copyright 2018 The Knative Authors

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

package kubernetesevents

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func TestUpdateEvent_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink         func(http.ResponseWriter, *http.Request)
		expectedBody string
	}{
		"happy": {
			sink:         sinkAccepted,
			expectedBody: `{"metadata":{"uid":"ABC","creationTimestamp":"%s"},"involvedObject":{"kind":"Kind","namespace":"Namespace","name":"Name","apiVersion":"api/version","resourceVersion":"v1test1","fieldPath":"field"},"source":{},"firstTimestamp":null,"lastTimestamp":null,"eventTime":null,"reportingComponent":"","reportingInstance":""}`,
		},
		// Kubernetes event adapter does not care if the sink accepts the event or not.
		"rejected": {
			sink:         sinkRejected,
			expectedBody: `{"metadata":{"uid":"ABC","creationTimestamp":"%s"},"involvedObject":{"kind":"Kind","namespace":"Namespace","name":"Name","apiVersion":"api/version","resourceVersion":"v1test1","fieldPath":"field"},"source":{},"firstTimestamp":null,"lastTimestamp":null,"eventTime":null,"reportingComponent":"","reportingInstance":""}`,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				SinkURI:   sinkServer.URL,
				Namespace: "default",
			}

			uid := types.UID("ABC")

			ref := corev1.ObjectReference{
				Kind:            "Kind",
				Namespace:       "Namespace",
				Name:            "Name",
				APIVersion:      "api/version",
				ResourceVersion: "v1test1",
				FieldPath:       "field",
			}

			now := time.Now()

			event := &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					UID:               uid,
					CreationTimestamp: metav1.Time{Time: now},
				},
				InvolvedObject: ref,
			}

			a.updateEvent(nil, event)

			expected := fmt.Sprintf(tc.expectedBody, now.UTC().Format(time.RFC3339))
			got := string(h.body)
			if expected != got {
				t.Errorf("expected message sent to be %v, got %v", expected, got)
			}
		})
	}
}

func TestUpdateEvent_NonExistentSink(t *testing.T) {
	a := &Adapter{
		SinkURI:   "http://localhost:50/",
		Namespace: "default",
	}

	uid := types.UID("ABC")

	ref := corev1.ObjectReference{
		Kind:            "Kind",
		Namespace:       "Namespace",
		Name:            "Name",
		APIVersion:      "api/version",
		ResourceVersion: "v1test1",
		FieldPath:       "field",
	}

	now := time.Now()

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			UID:               uid,
			CreationTimestamp: metav1.Time{Time: now},
		},
		InvolvedObject: ref,
	}

	a.updateEvent(nil, event)
}

func TestStartWithKubeClient(t *testing.T) {
	a := &Adapter{
		SinkURI:    "http://localhost:50/",
		Namespace:  "default",
		kubeClient: testclient.NewSimpleClientset(),
	}

	stop := make(chan struct{})
	go func() {
		if err := a.Start(stop); err != nil {
			log.Printf("failed to start, %v", err) //not expected to start.
		}
	}()
}

func TestStartWithoutKubeClient(t *testing.T) {
	a := &Adapter{
		SinkURI:   "http://localhost:50/",
		Namespace: "default",
	}

	stop := make(chan struct{})
	go func() {
		if err := a.Start(stop); err != nil {
			log.Printf("failed to start, %v", err) //not expected to start.
		}
	}()
}

type fakeHandler struct {
	body []byte

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}
