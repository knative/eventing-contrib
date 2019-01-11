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

// Package gcppubsub implements the GcpPubSubSource controller.
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sources.eventing.knative.dev,resources=gcppubsubsources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sources.eventing.knative.dev,resources=gcppubsubsources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=,resources=secrets,verbs=get;list;watch

// This is needed to be able to watch channels when delivering to a channel.
// +kubebuilder:rbac:groups=eventing.knative.dev,resources=channels,verbs=get;list;watch

// This is needed to be able to watch the sink when delivering messages directly.
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services;routes,verbs=get;list;watch
package gcppubsub
