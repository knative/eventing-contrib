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

package kuberneteseventsource

import (
	"fmt"
	"os"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	raImageEnvVar = "K8S_RA_IMAGE"
)

// reconciler reconciles a KubernetesEventSource object
type reconciler struct {
	client.Client
	recorder            record.EventRecorder
	scheme              *runtime.Scheme
	receiveAdapterImage string
}

var _ reconcile.Reconciler = &reconciler{}

// Add creates a new KubernetesEventSource Controller and adds it to the Manager
// with default RBAC. The Manager will set fields on the Controller and Start it
// when the Manager is Started.
func Add(mgr manager.Manager) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	return add(mgr, newReconciler(mgr, receiveAdapterImage))
}

func newReconciler(mgr manager.Manager, receiveAdapterImage string) reconcile.Reconciler {
	return &reconciler{
		Client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		recorder:            mgr.GetRecorder(controllerAgentName),
		receiveAdapterImage: receiveAdapterImage,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerAgentName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to KubernetesEventSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.KubernetesEventSource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to owned ContainerSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.ContainerSource{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &sourcesv1alpha1.KubernetesEventSource{},
	})
	if err != nil {
		return err
	}

	return nil
}
