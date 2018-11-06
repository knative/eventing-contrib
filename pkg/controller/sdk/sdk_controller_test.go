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

package sdk

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"testing"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
var depKey = types.NamespacedName{Name: "foo-kr", Namespace: "default"}

const timeout = time.Second * 5

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	instance := &duckv1alpha1.AddressableType{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	c = mgr.GetClient()

	p := newProvider(mgr)

	recFn, requests := SetupTestReconcile(newReconciler(p, mgr))
	g.Expect(p.add(mgr, recFn)).NotTo(gomega.HaveOccurred())
	defer close(StartTestManager(mgr, g))

	// Create the ContainerSources object and expect the Reconcile and Deployment to be created
	err = c.Create(context.TODO(), instance)
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	g.Expect(err).NotTo(gomega.HaveOccurred())
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	kr := &duckv1alpha1.KResource{}
	g.Eventually(func() error { return c.Get(context.TODO(), depKey, kr) }, timeout).
		Should(gomega.Succeed())

	// Delete the KResource and expect Reconcile to be called for KResource deletion
	g.Expect(c.Delete(context.TODO(), kr)).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
	g.Eventually(func() error { return c.Get(context.TODO(), depKey, kr) }, timeout).
		Should(gomega.Succeed())

	// Manually delete KResource since GC isn't enabled in the test control plane
	g.Expect(c.Delete(context.TODO(), kr)).To(gomega.Succeed())

}

func newProvider(mgr manager.Manager) *Provider {
	return &Provider{
		AgentName:  "SDK-Test",
		Parent:     &duckv1alpha1.AddressableType{},
		Owns:       []runtime.Object{&duckv1alpha1.KResource{}},
		Reconciler: newAddressableReconciler(mgr),
	}
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(p *Provider, mgr manager.Manager) reconcile.Reconciler {
	dynamicClient, _ := dynamic.NewForConfig(mgr.GetConfig())
	p.Reconciler.InjectClient(mgr.GetClient())
	p.Reconciler.InjectConfig(mgr.GetConfig())
	return &Reconciler{
		provider:      *p,
		recorder:      mgr.GetRecorder(p.AgentName),
		client:        mgr.GetClient(),
		dynamicClient: dynamicClient,
	}
}

func newAddressableReconciler(mgr manager.Manager) KnativeReconciler {
	return &reconciler{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	addr, ok := object.(*duckv1alpha1.AddressableType)
	if !ok {
		return object, nil
	}

	// See if the source has been deleted
	accessor, err := meta.Accessor(addr)
	if err != nil {
		return object, err
	}
	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		return object, nil
	}

	kr := &duckv1alpha1.KResource{ObjectMeta: metav1.ObjectMeta{Name: addr.Name + "-kr", Namespace: addr.Namespace}}
	if err := controllerutil.SetControllerReference(addr, kr, r.scheme); err != nil {
		return addr, err
	}
	err = r.client.Create(ctx, kr)
	return addr, err
}
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	return nil
}
