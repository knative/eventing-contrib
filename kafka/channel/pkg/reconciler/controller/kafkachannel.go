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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/system"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned"
	kafkaScheme "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned/scheme"
	kafkaclientsetinjection "knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/client"
	"knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/informers/messaging/v1alpha1/kafkachannel"
	listers "knative.dev/eventing-contrib/kafka/channel/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing-contrib/kafka/channel/pkg/reconciler/controller/resources"
	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "KafkaChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-ch-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled           = "ChannelReconciled"
	channelReconcileFailed      = "ChannelReconcileFailed"
	channelUpdateStatusFailed   = "ChannelUpdateStatusFailed"
	dispatcherDeploymentCreated = "DispatcherDeploymentCreated"
	dispatcherDeploymentFailed  = "DispatcherDeploymentFailed"
	dispatcherServiceCreated    = "DispatcherServiceCreated"
	dispatcherServiceFailed     = "DispatcherServiceFailed"

	dispatcherDeploymentName = "kafka-ch-dispatcher"
	dispatcherServiceName    = "kafka-ch-dispatcher"
)

func init() {
	// Add run types to the default Kubernetes Scheme so Events can be
	// logged for run types.
	_ = kafkaScheme.AddToScheme(scheme.Scheme)
}

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	*reconciler.Base

	dispatcherNamespace      string
	dispatcherDeploymentName string
	dispatcherServiceName    string
	dispatcherImage          string

	kafkaConfig      *utils.KafkaConfig
	kafkaConfigError error
	kafkaClientSet   kafkaclientset.Interface

	// Using a shared kafkaClusterAdmin does not work currently because of an issue with
	// Shopify/sarama, see https://github.com/Shopify/sarama/issues/1162.
	kafkaClusterAdmin sarama.ClusterAdmin

	kafkachannelLister   listers.KafkaChannelLister
	kafkachannelInformer cache.SharedIndexInformer
	deploymentLister     appsv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
	endpointsLister      corev1listers.EndpointsLister
	impl                 *controller.Impl
}

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
	serviceGVK    = corev1.SchemeGroupVersion.WithKind("Service")
)

type envConfig struct {
	Image string `envconfig:"DISPATCHER_IMAGE" required:"true"`
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Check that our Reconciler implements cache.ResourceEventHandler
var _ cache.ResourceEventHandler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	kafkaChannelInformer := kafkachannel.Get(ctx)
	deploymentInformer := deployment.Get(ctx)
	endpointsInformer := endpoints.Get(ctx)
	serviceInformer := service.Get(ctx)

	kafkaChannelClientSet := kafkaclientsetinjection.Get(ctx)

	dispatcherNamespace := system.Namespace()

	r := &Reconciler{
		Base:                     reconciler.NewBase(ctx, controllerAgentName, cmw),
		dispatcherNamespace:      dispatcherNamespace,
		dispatcherDeploymentName: dispatcherDeploymentName,
		dispatcherServiceName:    dispatcherServiceName,
		kafkachannelLister:       kafkaChannelInformer.Lister(),
		kafkachannelInformer:     kafkaChannelInformer.Informer(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		endpointsLister:          endpointsInformer.Lister(),
		kafkaClientSet:           kafkaChannelClientSet,
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		r.Logger.Panicf("unable to process Kafka channel's required environment variables: %v", err)
	}
	r.dispatcherImage = env.Image

	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	// Get and Watch the Kakfa config map and dynamically update Kafka configuration.
	if _, err := kubeclient.Get(ctx).CoreV1().ConfigMaps(system.Namespace()).Get("config-kafka", metav1.GetOptions{}); err == nil {
		cmw.Watch("config-kafka", r.updateKafkaConfig)
	} else if !apierrors.IsNotFound(err) {
		logger.With(zap.Error(err)).Fatal("Error reading ConfigMap 'config-kafka'")
	}

	r.Logger.Info("Setting up event handlers")
	kafkaChannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	// Set up watches for dispatcher resources we care about, since any changes to these
	// resources will affect our Channels. So, set up a watch here, that will cause
	// a global Resync for all the channels to take stock of their health when these change.
	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherDeploymentName),
		Handler:    r,
	})
	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(dispatcherNamespace, dispatcherServiceName),
		Handler:    r,
	})
	return r.impl
}

// cache.ResourceEventHandler implementation.
// These 3 functions just cause a Global Resync of the channels, because any changes here
// should be reflected onto the channels.
func (r *Reconciler) OnAdd(obj interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

func (r *Reconciler) OnUpdate(old, new interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

func (r *Reconciler) OnDelete(obj interface{}) {
	r.impl.GlobalResync(r.kafkachannelInformer)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the KafkaChannel resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the KafkaChannel resource with this namespace/name.
	original, err := r.kafkachannelLister.KafkaChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("KafkaChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	channel := original.DeepCopy()

	// Reconcile this copy of the KafkaChannel and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling KafkaChannel", zap.Error(reconcileErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("KafkaChannel reconciled")
		r.Recorder.Event(channel, corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update KafkaChannel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update KafkaChannel's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, kc *v1alpha1.KafkaChannel) error {
	kc.Status.InitializeConditions()

	logger := logging.FromContext(ctx)
	// Verify channel is valid.
	kc.SetDefaults(ctx)
	if err := kc.Validate(ctx); err != nil {
		logger.Error("Invalid kafka channel", zap.String("channel", kc.Name), zap.Error(err))
		return err
	}

	// See if the channel has been deleted.
	if kc.DeletionTimestamp != nil {
		// Do not attempt retrying creating the client because it might be a permanent error
		// in which case the finalizer will never get removed.
		if kafkaClusterAdmin, err := r.createClient(ctx, kc); err == nil && r.kafkaConfig != nil {
			if err := r.deleteTopic(ctx, kc, kafkaClusterAdmin); err != nil {
				return err
			}
		}
		removeFinalizer(kc)
		_, err := r.kafkaClientSet.MessagingV1alpha1().KafkaChannels(kc.Namespace).Update(kc)
		return err
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating Kafka.
	if err := r.ensureFinalizer(kc); err != nil {
		return err
	}

	if r.kafkaConfig == nil {
		if r.kafkaConfigError == nil {
			r.kafkaConfigError = errors.New("The config map 'config-kafka' does not exist")
		}
		kc.Status.MarkConfigFailed("MissingConfiguration", "%v", r.kafkaConfigError)
		return r.kafkaConfigError
	}

	kafkaClusterAdmin, err := r.createClient(ctx, kc)
	if err != nil {
		kc.Status.MarkConfigFailed("InvalidConfiguration", "Unable to build Kafka admin client for channel %s: %v", kc.Name, err)
		return err
	}

	kc.Status.MarkConfigTrue()

	// We reconcile the status of the Channel by looking at:
	// 1. Kafka topic used by the channel.
	// 2. Dispatcher Deployment for it's readiness.
	// 3. Dispatcher k8s Service for it's existence.
	// 4. Dispatcher endpoints to ensure that there's something backing the Service.
	// 5. K8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service.

	if err := r.createTopic(ctx, kc, kafkaClusterAdmin); err != nil {
		kc.Status.MarkTopicFailed("TopicCreateFailed", "error while creating topic: %s", err)
		return err
	}
	kc.Status.MarkTopicTrue()

	dispatcherNamespace := r.dispatcherNamespace

	// Make sure the dispatcher deployment exists and propagate the status to the Channel
	_, err = r.reconcileDispatcher(ctx, dispatcherNamespace, kc)
	if err != nil {
		return err
	}

	// Make sure the dispatcher service exists and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.reconcileDispatcherService(ctx, dispatcherNamespace, kc)
	if err != nil {
		return err
	}

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(r.dispatcherNamespace).Get(r.dispatcherServiceName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			kc.Status.MarkEndpointsFailed("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")
		} else {
			logger.Error("Unable to get the dispatcher endpoints", zap.Error(err))
			kc.Status.MarkEndpointsFailed("DispatcherEndpointsGetFailed", "Failed to get dispatcher endpoints")
		}
		return err
	}

	if len(e.Subsets) == 0 {
		logger.Error("No endpoints found for Dispatcher service", zap.Error(err))
		kc.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		return fmt.Errorf("there are no endpoints ready for Dispatcher service %s", r.dispatcherServiceName)
	}
	kc.Status.MarkEndpointsTrue()

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via ExternalName
	svc, err := r.reconcileChannelService(ctx, kc)
	if err != nil {
		kc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
		return err
	}
	kc.Status.MarkChannelServiceTrue()
	kc.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	// close the connection
	err = kafkaClusterAdmin.Close()
	if err != nil {
		logger.Error("Error closing the connection", zap.Error(err))
		return err
	}

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return nil
}

func (r *Reconciler) reconcileDispatcher(ctx context.Context, dispatcherNamespace string, kc *v1alpha1.KafkaChannel) (*appsv1.Deployment, error) {
	d, err := r.deploymentLister.Deployments(dispatcherNamespace).Get(dispatcherDeploymentName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			args := resources.DispatcherArgs{
				DispatcherNamespace: dispatcherNamespace,
				Image:               r.dispatcherImage,
			}
			expected := resources.MakeDispatcher(args)
			d, err := r.KubeClientSet.AppsV1().Deployments(dispatcherNamespace).Create(expected)
			if err == nil {
				r.Recorder.Event(kc, corev1.EventTypeNormal, dispatcherDeploymentCreated, "Dispatcher deployment created")
				kc.Status.PropagateDispatcherStatus(&d.Status)
			} else {
				logging.FromContext(ctx).Error("Unable to create the dispatcher deployment", zap.Error(err))
				r.Recorder.Eventf(kc, corev1.EventTypeWarning, dispatcherDeploymentFailed, "Failed to create the dispatcher deployment: %v", err)
				kc.Status.MarkServiceFailed("DispatcherDeploymentFailed", "Failed to create the dispatcher deployment: %v", err)
			}
			return d, err
		}

		logging.FromContext(ctx).Error("Unable to get the dispatcher deployment", zap.Error(err))
		kc.Status.MarkServiceUnknown("DispatcherDeploymentFailed", "Failed to get dispatcher deployment: %v", err)
		return nil, err
	}

	kc.Status.PropagateDispatcherStatus(&d.Status)
	return d, nil
}

func (r *Reconciler) reconcileDispatcherService(ctx context.Context, dispatcherNamespace string, kc *v1alpha1.KafkaChannel) (*corev1.Service, error) {
	svc, err := r.serviceLister.Services(dispatcherNamespace).Get(dispatcherDeploymentName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeDispatcherService(dispatcherNamespace)
			svc, err := r.KubeClientSet.CoreV1().Services(dispatcherNamespace).Create(expected)

			if err == nil {
				r.Recorder.Event(kc, corev1.EventTypeNormal, dispatcherServiceCreated, "Dispatcher service created")
				kc.Status.MarkServiceTrue()
			} else {
				logging.FromContext(ctx).Error("Unable to create the dispatcher service", zap.Error(err))
				r.Recorder.Eventf(kc, corev1.EventTypeWarning, dispatcherServiceFailed, "Failed to create the dispatcher service: %v", err)
				kc.Status.MarkServiceFailed("DispatcherServiceFailed", "Failed to create the dispatcher service: %v", err)
			}

			return svc, err
		}

		logging.FromContext(ctx).Error("Unable to get the dispatcher service", zap.Error(err))
		kc.Status.MarkServiceUnknown("DispatcherServiceFailed", "Failed to get dispatcher service: %v", err)
		return nil, err
	}

	kc.Status.MarkServiceTrue()
	return svc, nil
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, channel *v1alpha1.KafkaChannel) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(channel.Namespace).Get(resources.MakeChannelServiceName(channel.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(channel, resources.ExternalService(r.dispatcherNamespace, r.dispatcherServiceName))
			if err != nil {
				logger.Error("Failed to create the channel service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.KubeClientSet.CoreV1().Services(channel.Namespace).Create(svc)
			if err != nil {
				logger.Error("simpleFailed to create the channel service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		}
		logger.Error("Unable to get the channel service", zap.Error(err))
		return nil, err
	}
	// Check to make sure that the KafkaChannel owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, channel) {
		return nil, fmt.Errorf("kafkachannel: %s/%s does not own Service: %q", channel.Namespace, channel.Name, svc.Name)
	}
	return svc, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.KafkaChannel) (*v1alpha1.KafkaChannel, error) {
	kc, err := r.kafkachannelLister.KafkaChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(kc.Status, desired.Status) {
		return kc, nil
	}

	becomesReady := desired.Status.IsReady() && !kc.Status.IsReady()

	// Don't modify the informers copy.
	existing := kc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.kafkaClientSet.MessagingV1alpha1().KafkaChannels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(new.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("KafkaChannel %q became ready after %v", kc.Name, duration)
		if err := r.StatsReporter.ReportReady("KafkaChannel", kc.Namespace, kc.Name, duration); err != nil {
			r.Logger.Infof("Failed to record ready for KafkaChannel %q: %v", kc.Name, err)
		}
	}
	return new, err
}

func (r *Reconciler) createClient(ctx context.Context, kc *v1alpha1.KafkaChannel) (sarama.ClusterAdmin, error) {
	// We don't currently initialize r.kafkaClusterAdmin, hence we end up creating the cluster admin client every time.
	// This is because of an issue with Shopify/sarama. See https://github.com/Shopify/sarama/issues/1162.
	// Once the issue is fixed we should use a shared cluster admin client. Also, r.kafkaClusterAdmin is currently
	// used to pass a fake admin client in the tests.
	kafkaClusterAdmin := r.kafkaClusterAdmin
	if kafkaClusterAdmin == nil {
		var err error
		kafkaClusterAdmin, err = resources.MakeClient(controllerAgentName, r.kafkaConfig.Brokers)
		if err != nil {
			return nil, err
		}
	}
	return kafkaClusterAdmin, nil
}

func (r *Reconciler) createTopic(ctx context.Context, channel *v1alpha1.KafkaChannel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	logger := logging.FromContext(ctx)

	topicName := utils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	logger.Info("Creating topic on Kafka cluster", zap.String("topic", topicName))
	err := kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: channel.Spec.ReplicationFactor,
		NumPartitions:     channel.Spec.NumPartitions,
	}, false)
	if e, ok := err.(*sarama.TopicError); ok && e.Err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		logger.Error("Error creating topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		logger.Info("Successfully created topic", zap.String("topic", topicName))
	}
	return err
}

func (r *Reconciler) deleteTopic(ctx context.Context, channel *v1alpha1.KafkaChannel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	logger := logging.FromContext(ctx)

	topicName := utils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	logger.Info("Deleting topic on Kafka Cluster", zap.String("topic", topicName))
	err := kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		logger.Error("Error deleting topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		logger.Info("Successfully deleted topic", zap.String("topic", topicName))
	}
	return err
}

func (r *Reconciler) ensureFinalizer(channel *v1alpha1.KafkaChannel) error {
	finalizers := sets.NewString(channel.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(channel.Finalizers, finalizerName),
			"resourceVersion": channel.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.kafkaClientSet.MessagingV1alpha1().KafkaChannels(channel.Namespace).Patch(channel.Name, types.MergePatchType, patch)
	return err
}

func (r *Reconciler) updateKafkaConfig(configMap *corev1.ConfigMap) {
	r.Logger.Info("Reloading Kafka configuration")
	kafkaConfig, err := utils.GetKafkaConfig(configMap.Data)
	if err != nil {
		r.Logger.Errorw("Error reading Kafka configuration", zap.Error(err))
	}
	// For now just override the previous config.
	// Eventually the previous config should be snapshotted to delete Kafka topics
	r.kafkaConfig = kafkaConfig
	r.kafkaConfigError = err

	// Trigger global resync
	r.impl.GlobalResync(r.kafkachannelInformer)

}

func removeFinalizer(channel *v1alpha1.KafkaChannel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Delete(finalizerName)
	channel.Finalizers = finalizers.List()
}
