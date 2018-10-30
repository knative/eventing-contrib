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

package gcppubsub

import (
	"fmt"
	"os"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "gcp-pubsub-source-controller"

	// gcpPubSubEnabledEnvVar is used to determine if the GCP PubSub Source's controller should run.
	// It will only run if the environment variable is defined and has the value 'true'.
	gcpPubSubEnabledEnvVar = "ENABLE_GCPPUBSUB_SOURCE"

	raImageEnvVar          = "GCPPUBSUB_RA_IMAGE"
	raServiceAccountEnvVar = "GCPPUBSUB_RA_SERVICE_ACCOUNT"
)

// Add creates a new GcpPubSubSource Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager) error {
	if enabled, defined := os.LookupEnv(gcpPubSubEnabledEnvVar); !defined || enabled != "true" {
		return nil
	}
	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable '%s' not defined", raImageEnvVar)
	}
	raServiceAccount, defined := os.LookupEnv(raServiceAccountEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable '%s' not defined", raServiceAccountEnvVar)
	}

	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.GcpPubSubSource{},
		Owns:      []runtime.Object{&v1.Deployment{}},
		Reconciler: &reconciler{
			receiveAdapterImage:              raImage,
			receiveAdapterServiceAccountName: raServiceAccount,
		},
	}

	return p.Add(mgr)
}
