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

// Package kuberneteseventsource implements a KubernetesEventSource controller.
// +kubebuilder:rbac:groups=sources.eventing.knative.dev,resources=kuberneteseventsources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sources.eventing.knative.dev,resources=kuberneteseventsources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sources.eventing.knative.dev,resources=containersources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=events,verbs=get;list;watch;create;update;patch;delete
package kuberneteseventsource

const (
	controllerAgentName = "kuberneteseventsource-controller"
)
