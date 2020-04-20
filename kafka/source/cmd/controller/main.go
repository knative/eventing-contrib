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

	"os"

	"k8s.io/apimachinery/pkg/runtime/schema"
	bindingsv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/kafka/source/pkg/reconciler/binding"
	"knative.dev/eventing-contrib/kafka/source/pkg/reconciler/source"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/psbinding"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"
)

const (
	component = "kafka-controller"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	// List the types to validate.
	sourcesv1alpha1.SchemeGroupVersion.WithKind("KafkaSource"):   &sourcesv1alpha1.KafkaSource{},
	bindingsv1alpha1.SchemeGroupVersion.WithKind("KafkaBinding"): &bindingsv1alpha1.KafkaBinding{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"defaulting.webhook.kafka.sources.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// Here is where you would infuse the context with state
			// (e.g. attach a store with configmap data)
			return ctx
		},

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.kafka.sources.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		types,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			// Here is where you would infuse the context with state
			// (e.g. attach a store with configmap data)
			return ctx
		},

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		callbacks,
	)
}

func NewKafkaBindingWebhook(opts ...psbinding.ReconcilerOption) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		return psbinding.NewAdmissionController(ctx,

			// Name of the resource webhook.
			"kafkabindings.webhook.kafka.sources.knative.dev",

			// The path on which to serve the webhook.
			"/kafkabindings",

			// How to get all the Bindables for configuring the mutating webhook.
			binding.ListAll,

			// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
			func(ctx context.Context, _ psbinding.Bindable) (context.Context, error) {
				// Here is where you would infuse the context with state
				// (e.g. attach a store with configmap data)
				return ctx, nil
			},
			opts...,
		)
	}
}

func main() {
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "kafka-source-webhook",
		Port:        8443,
		SecretName:  "kafka-source-webhook-certs",
	})

	kfkSelector := psbinding.WithSelector(psbinding.ExclusionSelector)
	if os.Getenv("KAFKA_BINDING_SELECTION_MODE") == "inclusion" {
		kfkSelector = psbinding.WithSelector(psbinding.InclusionSelector)
	}

	sharedmain.WebhookMainWithContext(ctx, component,
		certificates.NewController,
		NewDefaultingAdmissionController,
		NewValidationAdmissionController,

		// For each binding we have a controller and a binding webhook.
		binding.NewController, NewKafkaBindingWebhook(kfkSelector),

		source.NewController,
	)
}
