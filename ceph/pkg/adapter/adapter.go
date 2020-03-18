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
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	ceph "knative.dev/eventing-contrib/ceph/pkg/apis/v1alpha1"
	"knative.dev/eventing-contrib/pkg/kncloudevents"
)

var theClient cloudevents.Client
var stopCh chan struct{}

// postMessage convert bucket notifications to knative events and sent them to knative
func postMessage(notification ceph.BucketNotification) error {
	eventTime, err := time.Parse(time.RFC3339, notification.EventTime)
	if err != nil {
		log.Printf("Failed to parse event timestamp, using local time. Error: %s", err.Error())
		eventTime = time.Now()
	}

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID(notification.ResponseElements.XAmzRequestID + notification.ResponseElements.XAmzID2)
	event.SetSource(notification.EventSource + "." + notification.AwsRegion + "." + notification.S3.Bucket.Name)
	event.SetType("com.amazonaws." + notification.EventName)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetSubject(notification.S3.Object.Key)
	event.SetTime(eventTime)
	event.SetData(notification)

	log.Printf("Sending CloudEvent: %v", event)

	_, _, err = theClient.Send(context.Background(), event)
	return err
}

// postHandler handles incoming bucket notifications from ceph
func postHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Printf("%s method not allowed", r.Method)
		http.Error(w, "405 Method Not Allowed", http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading message body: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var notifications ceph.BucketNotifications
	err = json.Unmarshal(body, &notifications)

	if err != nil {
		log.Printf("Failed to parse JSON: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("%d events found in message", len(notifications.Records))
	for _, notification := range notifications.Records {
		log.Printf("Received Ceph bucket notification: %+v", notification)
		if err := postMessage(notification); err == nil {
			log.Printf("Event %s was successfully posted to knative", notification.EventID)
		} else {
			log.Printf("Failed to post event %s: %s", notification.EventID, err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

// Start the ceph bucket notifications to knative adapter
func Start(sinkURI string, port string) {
	var err error
	theClient, err = kncloudevents.NewDefaultClient(sinkURI)
	if err != nil {
		log.Printf("Failed to configure knative sink: %s", err.Error())
		return
	}
	log.Printf("Successfully configure knative sink: %s", sinkURI)
	http.HandleFunc("/", postHandler)
	stopCh = make(chan struct{})
	go http.ListenAndServe(":"+port, nil)
	log.Println("Ceph to Knative adapter spawned HTTP server")
	<-stopCh
	log.Println("Ceph to Knative adapter terminated")
}

// Stop the ceph bucket notifications to knative adapter
func Stop() {
	close(stopCh)
}
