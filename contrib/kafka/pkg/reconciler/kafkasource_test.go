/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/knative/eventing-sources/pkg/reconciler/eventtype"

	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/kafka/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingsourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

const (
	raImage = "test-ra-image"

	pscData = "kafkaClientCreatorData"

	image      = "github.com/knative/test/image"
	sourceName = "test-kafka-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	brokerKind            = "Broker"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"
	addressableDNS        = "addressable.sink.svc.cluster.local"
	addressableURI        = "http://addressable.sink.svc.cluster.local/"
)

func init() {
	// Add types to scheme
	v1.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	eventingsourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

// TODO
type kafkaClientCreatorData struct {
	clientCreateErr error
	subExists       bool
	subExistsErr    error
	createSubErr    error
	deleteSubErr    error
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name:       "not a Kafka source",
			Reconciles: getNonKafkaSource(),
			InitialState: []runtime.Object{
				getNonKafkaSource(),
			},
		}, {
			Name: "cannot get sink URI",
			InitialState: []runtime.Object{
				getSource(),
			},
			WantPresent: []runtime.Object{
				getSourceWithNoSink(),
			},
			WantErrMsg: "sinks.duck.knative.dev \"testsink\" not found",
		}, {
			Name: "cannot create receive adapter",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot list deployments",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "successful create adapter",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			WantPresent: []runtime.Object{
				getReadySourceAndMarkEventTypes(),
			},
		}, {
			Name: "successful create - reuse existing receive adapter",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getReceiveAdapter(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadySourceAndMarkEventTypes(),
			},
		}, {
			Name: "successful create event types",
			InitialState: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if eventType, ok := obj.(*eventingv1alpha1.EventType); ok {
							// Hack because the fakeClient does not support GenerateName.
							if strings.Contains(eventType.Spec.Source, "topic1") {
								eventType.Name = "name1"
							} else {
								eventType.Name = "name2"
							}
							return controllertesting.Unhandled, nil
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadyAndMarkEventTypesSourceWithKind(brokerKind),
				getEventType("name1", "topic1"),
				getEventType("name2", "topic2"),
			},
		}, {
			Name: "successful create missing event types",
			InitialState: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
				getEventType("name2", "topic2"),
				getEventType("name3", "whatever_topic"),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if eventType, ok := obj.(*eventingv1alpha1.EventType); ok {
							// Hack because the fakeClient does not support GenerateName.
							if strings.Contains(eventType.Spec.Source, "topic1") {
								eventType.Name = "name1"
							}
							return controllertesting.Unhandled, nil
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadyAndMarkEventTypesSourceWithKind(brokerKind),
				getEventType("name1", "topic1"),
				getEventType("name2", "topic2"),
			},
			WantAbsent: []runtime.Object{
				getEventType("name3", "whatever_topic"),
			},
		}, {
			Name: "successful delete event type",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getReceiveAdapter(),
				getEventType("name1", "topic1"),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadySourceAndMarkEventTypes(),
			},
			WantAbsent: []runtime.Object{
				getEventType("name1", "topic1"),
			},
		}, {
			Name: "cannot create event types",
			InitialState: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object) (controllertesting.MockHandled, error) {
						if _, ok := obj.(*eventingv1alpha1.EventType); ok {
							return controllertesting.Handled, errors.New("test-induced-error")
						}
						return controllertesting.Unhandled, nil
					},
				},
			},
			WantAbsent: []runtime.Object{
				getEventType("name1", "topic1"),
				getEventType("name2", "topic2"),
			},
			WantPresent: []runtime.Object{
				getSourceWithSinkAndDeployedAndKind(brokerKind),
			},
			WantErrMsg: "test-induced-error",
		},
	}

	for _, tc := range testCases {
		tc.IgnoreTimes = true
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, sourceName)
		if tc.Reconciles == nil {
			tc.Reconciles = getSource()
		}
		tc.Scheme = scheme.Scheme

		c := tc.GetClient()
		r := &reconciler{
			client:              c,
			scheme:              tc.Scheme,
			receiveAdapterImage: raImage,
			eventTypeReconciler: eventtype.Reconciler{
				Scheme: tc.Scheme,
			},
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getNonKafkaSource() *eventingsourcesv1alpha1.ContainerSource {
	obj := &eventingsourcesv1alpha1.ContainerSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingsourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ContainerSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: eventingsourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args:  []string(nil),
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}

	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""

	return obj
}

func getSource() *sourcesv1alpha1.KafkaSource {
	return getSourceWithKind(addressableKind)
}

func getSourceWithKind(kind string) *sourcesv1alpha1.KafkaSource {
	obj := &sourcesv1alpha1.KafkaSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "KafkaSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.KafkaSourceSpec{
			BootstrapServers: "server1,server2",
			Topics:           "topic1,topic2",
			ConsumerGroup:    "group",
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       kind,
				APIVersion: addressableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getEventTypeForSource(name, topic string, source *sourcesv1alpha1.KafkaSource) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "EventType",
		},
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:               "KafkaSource",
					Name:               sourceName,
					Controller:         &trueVal,
					BlockOwnerDeletion: &trueVal,
					UID:                sourceUID,
				},
			},
			Name:         name,
			GenerateName: fmt.Sprintf("%s-", sourcesv1alpha1.KafkaEventType),
			Namespace:    testNS,
			Labels:       getLabels(source),
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   sourcesv1alpha1.KafkaEventType,
			Source: sourcesv1alpha1.KafkaEventSource(testNS, sourceName, topic),
			Broker: addressableName,
		},
	}
}

func getEventType(name, topic string) *eventingv1alpha1.EventType {
	return getEventTypeForSource(name, topic, getSourceWithKind(brokerKind))
}

func getSourceWithNoSink() *sourcesv1alpha1.KafkaSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkNoSink("NotFound", "")
	return src
}

func getSourceWithSink() *sourcesv1alpha1.KafkaSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSink(addressableURI)
	return src
}

func getSourceWithSinkAndDeployedAndKind(kind string) *sourcesv1alpha1.KafkaSource {
	src := getSourceWithKind(kind)
	src.Status.InitializeConditions()
	src.Status.MarkSink(addressableURI)
	src.Status.MarkDeployed()
	return src
}

func getReadySource() *sourcesv1alpha1.KafkaSource {
	src := getSourceWithSink()
	src.Status.MarkDeployed()
	return src
}

func getReadySourceWithKind(kind string) *sourcesv1alpha1.KafkaSource {
	src := getReadySource()
	src.Spec.Sink.Kind = kind
	return src
}

func getReadySourceAndMarkEventTypes() *sourcesv1alpha1.KafkaSource {
	src := getReadySource()
	src.Status.MarkEventTypes()
	return src
}

func getReadyAndMarkEventTypesSourceWithKind(kind string) *sourcesv1alpha1.KafkaSource {
	src := getReadySourceWithKind(kind)
	src.Status.MarkEventTypes()
	return src
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
		UID:       sourceUID,
	}
}

func getAddressable() *unstructured.Unstructured {
	return getAddressableWithKind(addressableKind)
}

func getAddressableWithKind(kind string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": addressableDNS,
				},
			},
		},
	}
}

func getReceiveAdapter() *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					Controller: &trueVal,
					UID:        sourceUID,
				},
			},
			Labels:    getLabels(getSource()),
			Namespace: testNS,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
	}
}
