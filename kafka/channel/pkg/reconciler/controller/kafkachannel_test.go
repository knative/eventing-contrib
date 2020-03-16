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

package controller

import (
	"context"
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/kmeta"

	. "knative.dev/eventing-contrib/kafka/channel/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clientgotesting "k8s.io/client-go/testing"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	fakekafkaclient "knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/client/fake"
	"knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/reconciler/messaging/v1alpha1/kafkachannel"
	"knative.dev/eventing-contrib/kafka/channel/pkg/reconciler/controller/resources"
	reconcilekafkatesting "knative.dev/eventing-contrib/kafka/channel/pkg/reconciler/testing"
	reconcilertesting "knative.dev/eventing-contrib/kafka/channel/pkg/reconciler/testing"
	eventingClient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	systemNS              = "knative-eventing"
	testNS                = "test-namespace"
	kcName                = "test-kc"
	testDispatcherImage   = "test-image"
	channelServiceAddress = "test-kc-kn-channel.test-namespace.svc.cluster.local"
	brokerName            = "test-broker"
	finalizerName         = "kafkachannels.messaging.knative.dev"
)

var (
	trueVal = true
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	finalizerUpdatedEvent = Eventf(corev1.EventTypeNormal, "FinalizerUpdate", `Updated "test-kc" finalizers`)
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCases(t *testing.T) {
	kcKey := testNS + "/" + kcName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "deleting",
			Key:  kcKey,
			Objects: []runtime.Object{
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaChannelDeleted)},
			WantErr: false,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "KafkaChannelReconciled", `KafkaChannel reconciled: "test-namespace/test-kc"`),
			},
		}, {
			Name: "deployment does not exist, automatically created and patching finalizers",
			Key:  kcKey,
			Objects: []runtime.Object{
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaChannelTopicReady()),
			},
			WantErr: true,
			WantCreates: []runtime.Object{
				makeDeployment(),
				makeService(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")),
			}},
			WantPatches: []clientgotesting.PatchActionImpl{
				patchFinalizers(testNS, kcName),
			},
			WantEvents: []string{
				finalizerUpdatedEvent,
				Eventf(corev1.EventTypeNormal, dispatcherDeploymentCreated, "Dispatcher deployment created"),
				Eventf(corev1.EventTypeNormal, dispatcherServiceCreated, "Dispatcher service created"),
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "kafka-ch-dispatcher" not found`),
			},
		}, {
			Name: "Service does not exist, automatically created",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantCreates: []runtime.Object{
				makeService(),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, dispatcherServiceCreated, "Dispatcher service created"),
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "kafka-ch-dispatcher" not found`),
			},
		}, {
			Name: "Endpoints does not exist",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "kafka-ch-dispatcher" not found`),
			},
		}, {
			Name: "Endpoints not ready",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeEmptyEndpoints(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsNotReady("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service"),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `there are no endpoints ready for Dispatcher service kafka-ch-dispatcher`),
			},
		}, {
			Name: "Works, creates new channel",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: false,
			WantCreates: []runtime.Object{
				makeChannelService(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
					reconcilekafkatesting.WithKafkaChannelChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "KafkaChannelReconciled", `KafkaChannel reconciled: "test-namespace/test-kc"`),
			},
		}, {
			Name: "Works, channel exists",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
				makeChannelService(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
			},
			WantErr: false,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
					reconcilekafkatesting.WithKafkaChannelChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelAddress(channelServiceAddress),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "KafkaChannelReconciled", `KafkaChannel reconciled: "test-namespace/test-kc"`),
			},
		}, {
			Name: "channel exists, not owned by us",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
				makeChannelServiceNotOwnedByUs(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
			},
			WantErr: true,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
					reconcilekafkatesting.WithKafkaChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: kafkachannel: test-namespace/test-kc does not own Service: \"test-kc-kn-channel\""),
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", `kafkachannel: test-namespace/test-kc does not own Service: "test-kc-kn-channel"`),
			},
		}, {
			Name: "channel does not exist, fails to create",
			Key:  kcKey,
			Objects: []runtime.Object{
				makeReadyDeployment(),
				makeService(),
				makeReadyEndpoints(),
				reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
			},
			WantErr: true,
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "Services"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
					reconcilekafkatesting.WithInitKafkaChannelConditions,
					reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
					reconcilekafkatesting.WithKafkaChannelConfigReady(),
					reconcilekafkatesting.WithKafkaChannelTopicReady(),
					reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
					reconcilekafkatesting.WithKafkaChannelServiceReady(),
					reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
					reconcilekafkatesting.WithKafkaChannelChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: inducing failure for create services"),
				),
			}},
			WantCreates: []runtime.Object{
				makeChannelService(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
			},
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create services"),
			},
			// TODO add UTs for topic creation and deletion.
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconcilertesting.MakeFactory(func(ctx context.Context, listers *reconcilekafkatesting.Listers, cmw configmap.Watcher) controller.Reconciler {

		r := &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			systemNamespace: testNS,
			dispatcherImage: testDispatcherImage,
			kafkaConfig: &KafkaConfig{
				Brokers: []string{brokerName},
			},
			kafkachannelLister: listers.GetKafkaChannelLister(),
			// TODO fix
			kafkachannelInformer: nil,
			deploymentLister:     listers.GetDeploymentLister(),
			serviceLister:        listers.GetServiceLister(),
			endpointsLister:      listers.GetEndpointsLister(),
			kafkaClusterAdmin:    &mockClusterAdmin{},
			kafkaClientSet:       fakekafkaclient.Get(ctx),
			KubeClientSet:        kubeclient.Get(ctx),
			EventingClientSet:    eventingClient.Get(ctx),
		}
		return kafkachannel.NewReconciler(ctx, r.Logger, r.kafkaClientSet, listers.GetKafkaChannelLister(), r.Recorder, r)
	}, zap.L()))
}

func TestTopicExists(t *testing.T) {
	kcKey := testNS + "/" + kcName
	row := TableRow{
		Name: "Works, topic already exists",
		Key:  kcKey,
		Objects: []runtime.Object{
			makeReadyDeployment(),
			makeService(),
			makeReadyEndpoints(),
			reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
				reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
		},
		WantErr: false,
		WantCreates: []runtime.Object{
			makeChannelService(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
				reconcilekafkatesting.WithInitKafkaChannelConditions,
				reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
				reconcilekafkatesting.WithKafkaChannelConfigReady(),
				reconcilekafkatesting.WithKafkaChannelTopicReady(),
				reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
				reconcilekafkatesting.WithKafkaChannelServiceReady(),
				reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
				reconcilekafkatesting.WithKafkaChannelChannelServiceReady(),
				reconcilekafkatesting.WithKafkaChannelAddress(channelServiceAddress),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "KafkaChannelReconciled", `KafkaChannel reconciled: "test-namespace/test-kc"`),
		},
	}
	defer logtesting.ClearAll()

	row.Test(t, reconcilertesting.MakeFactory(func(ctx context.Context, listers *reconcilekafkatesting.Listers, cmw configmap.Watcher) controller.Reconciler {

		r := &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			systemNamespace: testNS,
			dispatcherImage: testDispatcherImage,
			kafkaConfig: &KafkaConfig{
				Brokers: []string{brokerName},
			},
			kafkachannelLister: listers.GetKafkaChannelLister(),
			// TODO fix
			kafkachannelInformer: nil,
			deploymentLister:     listers.GetDeploymentLister(),
			serviceLister:        listers.GetServiceLister(),
			endpointsLister:      listers.GetEndpointsLister(),
			kafkaClusterAdmin: &mockClusterAdmin{
				mockCreateTopicFunc: func(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
					errMsg := sarama.ErrTopicAlreadyExists.Error()
					return &sarama.TopicError{
						Err:    sarama.ErrTopicAlreadyExists,
						ErrMsg: &errMsg,
					}
				},
			},
			kafkaClientSet:    fakekafkaclient.Get(ctx),
			KubeClientSet:     kubeclient.Get(ctx),
			EventingClientSet: eventingClient.Get(ctx),
		}
		return kafkachannel.NewReconciler(ctx, r.Logger, r.kafkaClientSet, listers.GetKafkaChannelLister(), r.Recorder, r)
	}, zap.L()))
}

func TestDeploymentUpdatedOnImageChange(t *testing.T) {
	kcKey := testNS + "/" + kcName
	row := TableRow{
		Name: "Works, topic already exists",
		Key:  kcKey,
		Objects: []runtime.Object{
			makeDeploymentWithImage("differentimage"),
			makeService(),
			makeReadyEndpoints(),
			reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
				reconcilekafkatesting.WithKafkaFinalizer(finalizerName)),
		},
		WantErr: false,
		WantCreates: []runtime.Object{
			makeChannelService(reconcilekafkatesting.NewKafkaChannel(kcName, testNS)),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: makeDeployment(),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: reconcilekafkatesting.NewKafkaChannel(kcName, testNS,
				reconcilekafkatesting.WithInitKafkaChannelConditions,
				reconcilekafkatesting.WithKafkaFinalizer(finalizerName),
				reconcilekafkatesting.WithKafkaChannelConfigReady(),
				reconcilekafkatesting.WithKafkaChannelTopicReady(),
				//				reconcilekafkatesting.WithKafkaChannelDeploymentReady(),
				reconcilekafkatesting.WithKafkaChannelServiceReady(),
				reconcilekafkatesting.WithKafkaChannelEndpointsReady(),
				reconcilekafkatesting.WithKafkaChannelChannelServiceReady(),
				reconcilekafkatesting.WithKafkaChannelAddress(channelServiceAddress),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, dispatcherDeploymentUpdated, "Dispatcher deployment updated"),
			Eventf(corev1.EventTypeNormal, "KafkaChannelReconciled", `KafkaChannel reconciled: "test-namespace/test-kc"`),
		},
	}
	defer logtesting.ClearAll()

	row.Test(t, reconcilertesting.MakeFactory(func(ctx context.Context, listers *reconcilekafkatesting.Listers, cmw configmap.Watcher) controller.Reconciler {

		r := &Reconciler{
			Base:            reconciler.NewBase(ctx, controllerAgentName, cmw),
			systemNamespace: testNS,
			dispatcherImage: testDispatcherImage,
			kafkaConfig: &KafkaConfig{
				Brokers: []string{brokerName},
			},
			kafkachannelLister: listers.GetKafkaChannelLister(),
			// TODO fix
			kafkachannelInformer: nil,
			deploymentLister:     listers.GetDeploymentLister(),
			serviceLister:        listers.GetServiceLister(),
			endpointsLister:      listers.GetEndpointsLister(),
			kafkaClusterAdmin: &mockClusterAdmin{
				mockCreateTopicFunc: func(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
					errMsg := sarama.ErrTopicAlreadyExists.Error()
					return &sarama.TopicError{
						Err:    sarama.ErrTopicAlreadyExists,
						ErrMsg: &errMsg,
					}
				},
			},
			kafkaClientSet:    fakekafkaclient.Get(ctx),
			KubeClientSet:     kubeclient.Get(ctx),
			EventingClientSet: eventingClient.Get(ctx),
		}
		return kafkachannel.NewReconciler(ctx, r.Logger, r.kafkaClientSet, listers.GetKafkaChannelLister(), r.Recorder, r)
	}, zap.L()))
}

type mockClusterAdmin struct {
	mockCreateTopicFunc func(topic string, detail *sarama.TopicDetail, validateOnly bool) error
	mockDeleteTopicFunc func(topic string) error
}

func (ca *mockClusterAdmin) CreateTopic(topic string, detail *sarama.TopicDetail, validateOnly bool) error {
	if ca.mockCreateTopicFunc != nil {
		return ca.mockCreateTopicFunc(topic, detail, validateOnly)
	}
	return nil
}

func (ca *mockClusterAdmin) Close() error {
	return nil
}

func (ca *mockClusterAdmin) DeleteTopic(topic string) error {
	if ca.mockDeleteTopicFunc != nil {
		return ca.mockDeleteTopicFunc(topic)
	}
	return nil
}

func (ca *mockClusterAdmin) DescribeTopics(topics []string) (metadata []*sarama.TopicMetadata, err error) {
	return nil, nil
}

func (ca *mockClusterAdmin) ListTopics() (map[string]sarama.TopicDetail, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) CreatePartitions(topic string, count int32, assignment [][]int32, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) DeleteRecords(topic string, partitionOffsets map[int32]int64) error {
	return nil
}

func (ca *mockClusterAdmin) DescribeConfig(resource sarama.ConfigResource) ([]sarama.ConfigEntry, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) AlterConfig(resourceType sarama.ConfigResourceType, name string, entries map[string]*string, validateOnly bool) error {
	return nil
}

func (ca *mockClusterAdmin) CreateACL(resource sarama.Resource, acl sarama.Acl) error {
	return nil
}

func (ca *mockClusterAdmin) ListAcls(filter sarama.AclFilter) ([]sarama.ResourceAcls, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) DeleteACL(filter sarama.AclFilter, validateOnly bool) ([]sarama.MatchingAcl, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) ListConsumerGroups() (map[string]string, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) DescribeConsumerGroups(groups []string) ([]*sarama.GroupDescription, error) {
	return nil, nil
}

func (ca *mockClusterAdmin) ListConsumerGroupOffsets(group string, topicPartitions map[string][]int32) (*sarama.OffsetFetchResponse, error) {
	return &sarama.OffsetFetchResponse{}, nil
}

func (ca *mockClusterAdmin) DescribeCluster() (brokers []*sarama.Broker, controllerID int32, err error) {
	return nil, 0, nil
}

// Delete a consumer group.
func (ca *mockClusterAdmin) DeleteConsumerGroup(group string) error {
	return nil
}

func makeDeploymentWithImage(image string) *appsv1.Deployment {
	return resources.MakeDispatcher(resources.DispatcherArgs{
		DispatcherNamespace: testNS,
		Image:               image,
	})
}

func makeDeployment() *appsv1.Deployment {
	return makeDeploymentWithImage(testDispatcherImage)
}

func makeReadyDeployment() *appsv1.Deployment {
	d := makeDeployment()
	d.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}
	return d
}

func makeService() *corev1.Service {
	return resources.MakeDispatcherService(testNS)
}

func makeChannelService(nc *v1alpha1.KafkaChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", kcName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(nc),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.%s", dispatcherName, testNS, utils.GetClusterDomainName()),
		},
	}
}

func makeChannelServiceNotOwnedByUs(nc *v1alpha1.KafkaChannel) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      fmt.Sprintf("%s-kn-channel", kcName),
			Labels: map[string]string{
				resources.MessagingRoleLabel: resources.MessagingRole,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.%s", dispatcherName, testNS, utils.GetClusterDomainName()),
		},
	}
}

func makeEmptyEndpoints() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Endpoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      dispatcherName,
		},
	}
}

func makeReadyEndpoints() *corev1.Endpoints {
	e := makeEmptyEndpoints()
	e.Subsets = []corev1.EndpointSubset{{Addresses: []corev1.EndpointAddress{{IP: "1.1.1.1"}}}}
	return e
}

func patchFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

func patchRemoveFinalizers(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":[],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}
