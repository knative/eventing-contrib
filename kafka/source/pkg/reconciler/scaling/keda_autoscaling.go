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

package scaling

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/logging"
	"knative.dev/serving/pkg/apis/autoscaling"
)

const (
	kedaAutoscalingAnnotationClass       = "keda.autoscaling.knative.dev"
	kedaCooldownPeriodAnnodationKey      = kedaAutoscalingAnnotationClass + "/cooldownPeriod"
	kedaPollingIntervalAnnodationKey     = kedaAutoscalingAnnotationClass + "/pollingInterval"
	kedaTriggerLagThresholdAnnodationKey = kedaAutoscalingAnnotationClass + "/trigger.lagThreshold"
)

// ScaleKafkaSourceWithKeda will scale Kafak Source if autoscaling annotations are present
func ScaleKafkaSourceWithKeda(ctx context.Context, ra *v1.Deployment, src *v1alpha1.KafkaSource, dcs dynamic.Interface) {
	// TPPD check for annotations before creating struct
	s := &KedaAutoscaling{
		DynamicClientSet: dcs,
	}
	s.scaleKafkaSource(ctx, ra, src)
}

// KedaAutoscaling uses KEDA ScaledObject to provide scaling for Kafka source deployment
type KedaAutoscaling struct {
	DynamicClientSet dynamic.Interface

	minReplicaCount     *int32
	maxReplicaCount     *int32
	cooldownPeriod      *int32
	pollingInterval     *int32
	triggerLagThreshold *int32
}

func (r *KedaAutoscaling) scaleKafkaSource(ctx context.Context, ra *v1.Deployment, src *v1alpha1.KafkaSource) {
	r.readScalingAnnotations(ctx, src)
	// no scaling annotatins so no scaling work needed
	if r.minReplicaCount == nil && r.maxReplicaCount == nil {
		return
	}
	_, error := r.deployKedaScaledObject(ctx, ra, src)
	if error != nil {
		// additional logging?
	}
}

func (r *KedaAutoscaling) deployKedaScaledObject(ctx context.Context, ra *v1.Deployment, src *v1alpha1.KafkaSource) (*unstructured.Unstructured, error) {
	logger := logging.FromContext(ctx).Desugar()
	deploymentName := ra.GetName()
	logger.Info("Got ra", zap.Any("receiveAdapter", ra))
	logger.Info("Got ra name "+deploymentName, zap.Any("deploymentName", deploymentName))
	namespace := src.Namespace
	name := src.Name
	gvk := schema.GroupVersionKind{
		Group:   "keda.k8s.io",
		Version: "v1alpha1",
		Kind:    "ScaledObject",
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	scaledObjectResourceInterface := r.DynamicClientSet.Resource(gvr).Namespace(namespace)
	if scaledObjectResourceInterface == nil {
		return nil, fmt.Errorf("unable to create dynamic client for ScaledObject")
	}
	scaledObjectUnstr, err := r.generateKedaScaledObjectUnstructured(ctx, ra, src)
	if err != nil {
		return nil, err
	}
	created, err := scaledObjectResourceInterface.Create(scaledObjectUnstr, metav1.CreateOptions{})
	if err != nil {
		logger.Error("Failed to create ScaledObject so going to do update", zap.Error(err))
		//fmt.Printf("Doing update as failed to create ScaledObject: %s \n", err)
		// will replace - https://github.com/kubernetes/client-go/blob/master/examples/dynamic-create-update-delete-deployment/main.go
		// doing kubectl "replace" https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
		// first get resourceVersion
		existing, err := scaledObjectResourceInterface.Get(name, metav1.GetOptions{})
		if err != nil {
			logger.Error("Failed to create ScaledObject:", zap.Error(err))
			return nil, err
		}
		resourceVersion := existing.GetResourceVersion()
		scaledObjectUnstr.SetResourceVersion(resourceVersion)
		updated, err := scaledObjectResourceInterface.Update(scaledObjectUnstr, metav1.UpdateOptions{})
		if err != nil {
			logger.Error("Update failed to create ScaledObject", zap.Error(err))
			return nil, err
		} else {
			logger.Info("Update success", zap.Any("updated", updated))
			return updated, nil
		}
	}
	return created, nil
}

func convertMapKeyToInt32(dict map[string]string, key string, logger *zap.Logger) *int32 {
	val, ok := dict[key]
	if !ok {
		return nil
	}
	i, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		logger.Error("Expected annotation value to be integer but got "+val, zap.Any("annotations key", key))
		return nil
	}
	i32 := int32(i)
	return &i32
}

func (r *KedaAutoscaling) readScalingAnnotations(ctx context.Context, src *v1alpha1.KafkaSource) {
	logger := logging.FromContext(ctx).Desugar()
	meta := src.GetObjectMeta()
	annotations := meta.GetAnnotations()
	if annotations != nil {
		scalingClass := annotations[autoscaling.ClassAnnotationKey]
		r.minReplicaCount = convertMapKeyToInt32(annotations, autoscaling.MinScaleAnnotationKey, logger)
		r.maxReplicaCount = convertMapKeyToInt32(annotations, autoscaling.MaxScaleAnnotationKey, logger)
		if scalingClass == kedaAutoscalingAnnotationClass {
			r.cooldownPeriod = convertMapKeyToInt32(annotations, kedaCooldownPeriodAnnodationKey, logger)
			r.pollingInterval = convertMapKeyToInt32(annotations, kedaPollingIntervalAnnodationKey, logger)
			r.pollingInterval = convertMapKeyToInt32(annotations, kedaPollingIntervalAnnodationKey, logger)
			r.triggerLagThreshold = convertMapKeyToInt32(annotations, kedaTriggerLagThresholdAnnodationKey, logger)

		}
	}
}

func (r *KedaAutoscaling) generateKedaScaledObjectUnstructured(ctx context.Context, ra *v1.Deployment, src *v1alpha1.KafkaSource) (*unstructured.Unstructured, error) {
	logger := logging.FromContext(ctx).Desugar()
	deploymentName := ra.GetName()
	namespace := src.Namespace
	name := src.Name
	logger.Info("Unstructured ScaledObject name "+name, zap.Any("name", name))
	srcName := src.GetName()
	srcUID := src.GetUID()
	srcResVersion := src.GetResourceVersion()
	logger.Info("Got srcResVersion="+srcResVersion, zap.Any("srcResVersion", srcResVersion))
	srcKind := src.GetGroupVersionKind().Kind
	logger.Info("Got srcKind srcName srcUID", zap.Any("srcKind", srcKind), zap.Any("srcName", srcName), zap.Any("srcUID", srcUID))
	srcBrokerList := src.Spec.BootstrapServers
	srcConcumerGroup := src.Spec.ConsumerGroup
	triggers := make([]map[string]interface{}, 0, 1)
	topics := strings.Split(src.Spec.Topics, ",")
	if len(topics) == 0 {
		return nil, fmt.Errorf("Comma-separated list of topics can not be empty")
	}
	for _, topic := range topics {
		triggerMetadata := map[string]interface{}{
			"brokerList":    srcBrokerList,
			"consumerGroup": srcConcumerGroup,
			"topic":         topic,
		}
		if r.triggerLagThreshold != nil {
			logger.Info("Got triggerLagThreshold", zap.Any("triggerLagThreshold", r.triggerLagThreshold))
			triggerMetadata["lagThreshold"] = strconv.Itoa(int(*r.triggerLagThreshold))
		}
		trigger := map[string]interface{}{
			"type":     "kafka",
			"metadata": triggerMetadata,
		}
		triggers = append(triggers, trigger)
	}
	spec := map[string]interface{}{
		"scaleTargetRef": map[string]interface{}{
			"deploymentName": deploymentName,
		},
		"triggers": triggers,
	}
	if r.minReplicaCount != nil {
		logger.Info("Got minReplicaCount", zap.Any("minReplicaCount", r.minReplicaCount))
		spec["minReplicaCount"] = *r.minReplicaCount
	}
	if r.maxReplicaCount != nil {
		logger.Info("Got maxReplicaCount", zap.Any("maxReplicaCount", r.maxReplicaCount))
		spec["maxReplicaCount"] = *r.maxReplicaCount
	}
	if r.cooldownPeriod != nil {
		logger.Info("Got cooldownPeriod", zap.Any("cooldownPeriod", r.cooldownPeriod))
		spec["cooldownPeriod"] = *r.cooldownPeriod
	}
	if r.pollingInterval != nil {
		logger.Info("Got pollingInterval", zap.Any("pollingInterval", r.minReplicaCount))
		spec["pollingInterval"] = *r.pollingInterval
	}
	soUnstr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.k8s.io/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"creationTimestamp": nil,
				"namespace":         namespace,
				"name":              name,
				"labels": map[string]interface{}{
					"deploymentName": deploymentName,
				},
				"ownerReferences": []map[string]interface{}{{
					"apiVersion":         "sources.eventing.knative.dev/v1alpha1",
					"kind":               srcKind,
					"name":               srcName,
					"uid":                srcUID,
					"blockOwnerDeletion": true,
					"controller":         true,
				}},
			},
			"spec": spec,
		},
	}
	logger.Info("Unstructured SO name "+name, zap.Any("name", name), zap.Any("soUnstr", soUnstr))
	return soUnstr, nil
}
