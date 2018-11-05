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

package githubsource

import (
	"fmt"
	"os"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "github-source-controller"
	raImageEnvVar       = "GH_RA_IMAGE"
)

// Add creates a new GitHubSource Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the
// Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &sourcesv1alpha1.GitHubSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			recorder:            mgr.GetRecorder(controllerAgentName),
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: receiveAdapterImage,
			webhookCreator:      gitHubWebhookCreator,
		},
	}

	return p.Add(mgr)
}
