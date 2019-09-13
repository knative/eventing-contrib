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

package main

import (
	"context"
	"flag"
	"log"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	messagingv1alpha1 "knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"
)

func main() {
	flag.Parse()
	// Read the logging config and setup a logger.
	cm, err := configmap.Load("/etc/config-logging")
	if err != nil {
		log.Fatalf("Error loading logging configuration: %v", err)
	}
	config, err := logging.NewConfigFromMap(cm)
	if err != nil {
		log.Fatalf("Error parsing logging configuration: %v", err)
	}
	logger, atomicLevel := logging.NewLoggerFromConfig(config, logconfig.WebhookName())
	defer logger.Sync()
	logger = logger.With(zap.String(logkey.ControllerType, logconfig.WebhookName()))

	logger.Infow("Starting the Kafka Messaging Webhook")

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatalw("Failed to get in cluster config", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		logger.Fatalw("Failed to get the client set", zap.Error(err))
	}

	// Watch the logging config map and dynamically update logging levels.
	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	configMapWatcher.Watch(logging.ConfigMapName(), logging.UpdateLevelFromConfigMap(logger, atomicLevel, logconfig.WebhookName()))

	stats, err := webhook.NewStatsReporter()
	if err != nil {
		logger.Fatalw("failed to initialize the stats reporter", zap.Error(err))
	}

	options := webhook.ControllerOptions{
		ServiceName:                     logconfig.WebhookName(),
		DeploymentName:                  logconfig.WebhookName(),
		Namespace:                       system.Namespace(),
		Port:                            8443,
		SecretName:                      "messaging-webhook-certs",
		StatsReporter:                   stats,
		ResourceMutatingWebhookName:     "webhook.messaging.knative.dev",
		ResourceAdmissionControllerPath: "/",
	}

	resourceHandlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		// For group messaging.knative.dev
		messagingv1alpha1.SchemeGroupVersion.WithKind("KafkaChannel"): &messagingv1alpha1.KafkaChannel{},
	}

	// Decorate contexts with the current state of the config.
	ctxFunc := func(ctx context.Context) context.Context {
		// TODO: implement upgrades when eventing needs it:
		//  return v1beta1.WithUpgradeViaDefaulting(store.ToContext(ctx))
		return ctx
	}

	resourceAdmissionController := webhook.NewResourceAdmissionController(resourceHandlers, options, true)
	admissionControllers := map[string]webhook.AdmissionController{
		options.ResourceAdmissionControllerPath: resourceAdmissionController,
	}

	controller, err := webhook.New(kubeClient, options, admissionControllers, logger, ctxFunc)
	if err != nil {
		logger.Fatalw("Failed to create the Kafka admission controller", zap.Error(err))
	}
	if err = controller.Run(stopCh); err != nil {
		logger.Errorw("controller.Run() failed", zap.Error(err))
	}
	logger.Infow("Kafka webhook stopping")
}
