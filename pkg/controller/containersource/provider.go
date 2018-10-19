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

package containersource

import (
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "container-source-controller"
)

// Add creates a new ContainerSource Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager) error {
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.ContainerSource{},
		Owns:      []runtime.Object{&appsv1.Deployment{}},
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			scheme:   mgr.GetScheme(),
		},
	}

	return p.Add(mgr)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {

	// TODO: The tests needs this to run...
	return nil
	//return &reconciler{
	//	client:   mgr.GetClient(),
	//	recorder: mgr.GetRecorder(controllerAgentName),
	//	scheme:   mgr.GetScheme(),
	//}
	//return &ReconcileContainerSource{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}
