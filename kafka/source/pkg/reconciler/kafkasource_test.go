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
	//	"errors"
	"fmt"
	//	"strings"
	"testing"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	//	controllertesting "knative.dev/eventing-contrib/pkg/controller/testing"
	//	"knative.dev/eventing-contrib/pkg/reconciler/eventtype"
	kafkaclient "knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned/fake"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingsourcesv1alpha1 "knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/configmap"
	//	"sigs.k8s.io/controller-runtime/pkg/client"

	clientgotesting "k8s.io/client-go/testing"
	. "knative.dev/eventing-contrib/kafka/source/pkg/reconciler/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

var (
	availableDeployment = &v1.Deployment{
		Status: v1.DeploymentStatus{
			Conditions: []v1.DeploymentCondition{
				{
					Type:   v1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
)

const (
	raImage = "test-ra-image"

	pscData = "kafkaClientCreatorData"

	image      = "knative.dev/test/image"
	sourceName = "test-kafka-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	generation = 1

	addressableName       = "testsink"
	addressableKind       = "Sink"
	brokerKind            = "Broker"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"
	addressableDNS        = "addressable.sink.svc.cluster.local"
	addressableURI        = "http://addressable.sink.svc.cluster.local"
)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
	_ = eventingsourcesv1alpha1.AddToScheme(scheme.Scheme)
	_ = sourcesv1alpha1.AddToScheme(scheme.Scheme)
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
	testCases := TableTest{
		{
			Name:    "cannot get sink URI",
			WantErr: true,
			Objects: []runtime.Object{
				NewKafkaSource(sourceName, testNS,
					WithKafkaSourceSpec(sourcesv1alpha1.KafkaSourceSpec{
						BootstrapServers: "server1,server2",
						Topics:           "topic1,topic2",
						ConsumerGroup:    "group",
						Sink: &corev1.ObjectReference{
							Name:       addressableName,
							Kind:       addressableKind,
							APIVersion: addressableAPIVersion,
						},
					}),
					WithKafkaSourceUID(sourceUID),
					WithKafkaSourceObjectMetaGeneration(generation),
				),
			},
			Key: testNS + "/" + sourceName,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewKafkaSource(sourceName, testNS,
					WithKafkaSourceSpec(sourcesv1alpha1.KafkaSourceSpec{
						BootstrapServers: "server1,server2",
						Topics:           "topic1,topic2",
						ConsumerGroup:    "group",
						Sink: &corev1.ObjectReference{
							Name:       addressableName,
							Kind:       addressableKind,
							APIVersion: addressableAPIVersion,
						},
					}),
					WithKafkaSourceUID(sourceUID),
					WithKafkaSourceObjectMetaGeneration(generation),
					// Status Update follows:
					WithInitKafkaSourceConditions,
					WithKafkaSourceStatusObservedGeneration(0), //This should be 1?
					WithKafkaSourceSinkNotFound,
				),
			}},
		},
		//		}, {
		/*			Name: "cannot create receive adapter",
			Objects: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := getSourceWithSink()
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot list deployments",
			Objects: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ runtime.Object, _ ...client.ListOption) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := getSourceWithSink()
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "successful create adapter",
			Objects: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := getReadySourceAndMarkEventTypes()
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
		}, {
			Name: "successful create - reuse existing receive adapter",
			Objects: []runtime.Object{
				getSource(),
				getAddressable(),
				getReceiveAdapter(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := getReadySourceAndMarkEventTypes()
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
		}, {
			Name: "successful create event types",
			Objects: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
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
				func() runtime.Object {
					s := getReadyAndMarkEventTypesSourceWithKind(brokerKind)
					s.Status.ObservedGeneration = generation
					return s
				}(),
				getEventType("name1", "topic1"),
				getEventType("name2", "topic2"),
			},
		}, {
			Name: "successful create missing event types",
			Objects: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
				getEventType("name2", "topic2"),
				getEventType("name3", "whatever_topic"),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
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
				func() runtime.Object {
					s := getReadyAndMarkEventTypesSourceWithKind(brokerKind)
					s.Status.ObservedGeneration = generation
					return s
				}(),

				getEventType("name1", "topic1"),
				getEventType("name2", "topic2"),
			},
			WantAbsent: []runtime.Object{
				getEventType("name3", "whatever_topic"),
			},
		}, {
			Name: "successful delete event type",
			Objects: []runtime.Object{
				getSource(),
				getAddressable(),
				getReceiveAdapter(),
				getEventType("name1", "topic1"),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := getReadySourceAndMarkEventTypes()
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantAbsent: []runtime.Object{
				getEventType("name1", "topic1"),
			},
		}, {
			Name: "cannot create event types",
			Objects: []runtime.Object{
				getSourceWithKind(brokerKind),
				getAddressableWithKind(brokerKind),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, obj runtime.Object, _ ...client.CreateOption) (controllertesting.MockHandled, error) {
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
				func() runtime.Object {
					s := getSourceWithSinkAndDeployedAndKind(brokerKind)
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantErrMsg: "test-induced-error",
		},*/
	}

	defer logtesting.ClearAll()
	for _, tc := range testCases {
		cs := kafkaclient.NewSimpleClientset(tc.Objects...)
		tc.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {

			r := &Reconciler{
				Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
				kafkaClientSet:      cs,
				kafkaLister:         listers.GetKafkaSourceLister(),
				deploymentLister:    listers.GetDeploymentLister(),
				eventTypeLister:     listers.GetEventTypeLister(),
				receiveAdapterImage: raImage,
			}
			r.sinkReconciler = duck.NewSinkReconciler(ctx, func(string) {})
			return r
		},
			true,
		))
	}
}

func getNonKafkaSource() *eventingsourcesv1alpha1.ContainerSource {
	obj := &eventingsourcesv1alpha1.ContainerSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: eventingsourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ContainerSource",
		},
		ObjectMeta: om(testNS, sourceName, generation),
		Spec: eventingsourcesv1alpha1.ContainerSourceSpec{
			Template: &corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: image,
						Args:  []string(nil),
					},
					},
				},
			},
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}

	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	obj.Status.InitializeConditions()
	obj.Status.ObservedGeneration = 0
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
		ObjectMeta: om(testNS, sourceName, generation),
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
	obj.Status.MarkResourcesCorrect()
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
	src.Status.MarkDeployed(availableDeployment)
	return src
}

func getReadySource() *sourcesv1alpha1.KafkaSource {
	src := getSourceWithSink()
	src.Status.MarkDeployed(availableDeployment)
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

func om(namespace, name string, generation int64) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:  namespace,
		Name:       name,
		Generation: generation,
		SelfLink:   fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
		UID:        sourceUID,
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
