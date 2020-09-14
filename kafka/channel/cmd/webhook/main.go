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

	"knative.dev/pkg/webhook/resourcesemantics/conversion"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/logconfig"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	messagingv1alpha1 "knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	messagingv1beta1 "knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// For group messaging.knative.dev
	messagingv1alpha1.SchemeGroupVersion.WithKind("KafkaChannel"): &messagingv1alpha1.KafkaChannel{},
	messagingv1beta1.SchemeGroupVersion.WithKind("KafkaChannel"):  &messagingv1beta1.KafkaChannel{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"defaulting.webhook.kafka.messaging.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"validation.webhook.kafka.messaging.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		callbacks,
	)
}

func NewConversionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	var (
		messagingv1alpha1_ = messagingv1alpha1.SchemeGroupVersion.Version
		messagingv1beta1_  = messagingv1beta1.SchemeGroupVersion.Version
	)

	return conversion.NewConversionController(ctx,
		// The path on which to serve the webhook
		"/resource-conversion",

		// Specify the types of custom resource definitions that should be converted
		map[schema.GroupKind]conversion.GroupKindConversion{
			// KafkaChannel
			messagingv1beta1.Kind("KafkaChannel"): {
				DefinitionName: messaging.KafkaChannelsResource.String(),
				HubVersion:     messagingv1alpha1_,
				Zygotes: map[string]conversion.ConvertibleObject{
					messagingv1alpha1_: &messagingv1alpha1.KafkaChannel{},
					messagingv1beta1_:  &messagingv1beta1.KafkaChannel{},
				},
			},
		},

		// A function that infuses the context passed to ConvertTo/ConvertFrom/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: logconfig.WebhookName(),
		Port:        webhook.PortFromEnv(8443),
		SecretName:  "messaging-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, "kafkachannel-webhook",
		certificates.NewController,
		NewDefaultingAdmissionController,
		NewValidationAdmissionController,
		NewConversionController,
		// TODO(mattmoor): Support config validation in eventing-contrib.
	)
}
