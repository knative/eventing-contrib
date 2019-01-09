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

package awssqssource

import (
	"fmt"
	"log"
	"os"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to
	// identify itself when creating events.
	controllerAgentName = "aws-sqs-source-controller"

	// awsSqsEnabledEnvVar is used to determine if the AWS SQS Source's
	// controller should run.  It will only run if the environment
	// variable is defined and has the value 'true'.
	awsSqsEnabledEnvVar = "ENABLE_AWSSQS_SOURCE"

	// raImageEnvVar is the name of the environment variable that
	// contains the receive adapter's image. It must be defined.
	raImageEnvVar = "AWSSQS_RA_IMAGE"
)

// Add creates a new AwsSqsSource Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager) error {
	if enabled, defined := os.LookupEnv(awsSqsEnabledEnvVar); !defined || enabled != "true" {
		log.Println("Skipping the AWS SQS Source controller.")
		return nil
	}
	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable '%s' not defined", raImageEnvVar)
	}

	log.Println("Adding the AWS SQS Source controller.")
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.AwsSqsSource{},
		Owns:      []runtime.Object{&v1.Deployment{}},
		Reconciler: &reconciler{
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: raImage,
		},
	}

	return p.Add(mgr)
}
