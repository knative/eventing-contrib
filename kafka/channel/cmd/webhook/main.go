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

	"k8s.io/apimachinery/pkg/runtime/schema"
	messagingv1alpha1 "knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
)

func NewResourceAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	// Decorate contexts with the current state of the config.
	// store := defaultconfig.NewStore(logging.FromContext(ctx).Named("config-store"))
	// store.WatchConfigs(cmw)
	ctxFunc := func(ctx context.Context) context.Context {
		// return v1.WithUpgradeViaDefaulting(store.ToContext(ctx))
		return ctx
	}

	return resourcesemantics.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"webhook.kafka.messaging.knative.dev",

		// The path on which to serve the webhook.
		"/",

		// The resources to validate and default.
		map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
			// For group messaging.knative.dev
			messagingv1alpha1.SchemeGroupVersion.WithKind("KafkaChannel"): &messagingv1alpha1.KafkaChannel{},
		},

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        8443,
		SecretName:  "messaging-webhook-certs",
	})

	sharedmain.MainWithContext(ctx, "kafkachannel_webhook",
		certificates.NewController,
		NewResourceAdmissionController,
		// TODO(mattmoor): Support config validation in eventing-contrib.
	)
}
