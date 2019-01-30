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
	"strings"

	"github.com/knative/pkg/cloudevents"
	corev1 "k8s.io/api/core/v1"
)

// Creates a URI of the form found in object metadata selfLinks
// Format looks like: /apis/feeds.knative.dev/v1alpha1/namespaces/default/feeds/k8s-events-example
// KNOWN ISSUES:
// * ObjectReference.APIVersion has no version information (e.g. serving.knative.dev rather than serving.knative.dev/v1alpha1)
// * ObjectReference does not have enough information to create the pluaralized list type (e.g. "revisions" from kind: Revision)
//
// Track these issues at https://github.com/kubernetes/kubernetes/issues/66313
// We could possibly work around this by adding a lister for the resources referenced by these events.
func createSelfLink(o corev1.ObjectReference) string {
	collectionNameHack := strings.ToLower(o.Kind) + "s"
	versionNameHack := o.APIVersion

	// Core API types don't have a separate package name and only have a version string (e.g. /apis/v1/namespaces/default/pods/myPod)
	// To avoid weird looking strings like "v1/versionUnknown" we'll sniff for a "." in the version
	if strings.Contains(versionNameHack, ".") && !strings.Contains(versionNameHack, "/") {
		versionNameHack = versionNameHack + "/versionUnknown"
	}
	return fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s", versionNameHack, o.Namespace, collectionNameHack, o.Name)
}

// Creates a CloudEvent Context for a given K8S event. For clarity, the following is a spew-dump
// of a real K8S event:
//	&Event{
//		ObjectMeta:k8s_io_apimachinery_pkg_apis_meta_v1.ObjectMeta{
//			Name:k8s-events-00001.1542495ae3f5fbba,
//			Namespace:default,
//			SelfLink:/api/v1/namespaces/default/events/k8s-events-00001.1542495ae3f5fbba,
//			UID:fc2fffb3-8a12-11e8-8874-42010a8a0fd9,
//			ResourceVersion:2729,
//			Generation:0,
//			CreationTimestamp:2018-07-17 22:44:37 +0000 UTC,
//		},
//		InvolvedObject:ObjectReference{
//			Kind:Revision,
//			Namespace:default,
//			Name:k8s-events-00001,
//			UID:f5c19306-8a12-11e8-8874-42010a8a0fd9,
//			APIVersion:serving.knative.dev,
//			ResourceVersion:42683,
//		},
//		Reason:RevisionReady,
//		Message:Revision becomes ready upon endpoint "k8s-events-00001-service" becoming ready,
//		Source:EventSource{
//			Component:revision-controller,
//		},
//		FirstTimestamp:2018-07-17 22:44:37 +0000 UTC,
//		LastTimestamp:2018-07-17 22:49:40 +0000 UTC,
//		Count:91,
//		Type:Normal,
//		EventTime:0001-01-01 00:00:00 +0000 UTC,
//	}
func cloudEventOverrides(m *corev1.Event) cloudevents.V01EventContext {
	return cloudevents.V01EventContext{
		EventID:   string(m.ObjectMeta.UID),
		Source:    createSelfLink(m.InvolvedObject),
		EventTime: m.ObjectMeta.CreationTimestamp.Time,
	}
}
