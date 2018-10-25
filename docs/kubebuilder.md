The following is notes how we use kubebuilder and changes required from the
defaults.

# Making EdgeSource

```
⦿ kubebuilder create api --group sources --version v1alpha1 --kind EdgeSource
Create Resource under pkg/apis [y/n]?
y
Create Controller under pkg/controller [y/n]?
y
Writing scaffold for you to edit...
pkg/apis/sources/v1alpha1/edgesource_types.go
pkg/apis/sources/v1alpha1/edgesource_types_test.go
pkg/controller/edgesource/edgesource_controller.go
pkg/controller/edgesource/edgesource_controller_test.go
Running make...
./hack/update-deps.sh
./hack/update-codegen.sh
Generating deepcopy funcs
Generating clientset for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/clientset
Generating listers for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/listers
Generating informers for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/informers
./hack/update-manifests.sh
CRD manifests generated under '/Users/nicholss/go/src/github.com/knative/eventing-sources/config/crds'
RBAC manifests generated under '/Users/nicholss/go/src/github.com/knative/eventing-sources/config/rbac'
./hack/verify-codegen.sh
Generating deepcopy funcs
Generating clientset for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/clientset
Generating listers for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/listers
Generating informers for sources:v1alpha1 at github.com/knative/eventing-sources/pkg/client/informers
Diffing /Users/nicholss/go/src/github.com/knative/eventing-sources against freshly generated codegen
/Users/nicholss/go/src/github.com/knative/eventing-sources up to date.
./hack/verify-manifests.sh
CRD manifests generated under '/Users/nicholss/go/src/github.com/knative/eventing-sources/config/crds'
RBAC manifests generated under '/Users/nicholss/go/src/github.com/knative/eventing-sources/config/rbac'
Diffing /Users/nicholss/go/src/github.com/knative/eventing-sources against freshly generated manifests
/Users/nicholss/go/src/github.com/knative/eventing-sources up to date.
go test ./pkg/... ./cmd/... -coverprofile cover.out
?   	github.com/knative/eventing-sources/pkg/apis	[no test files]
?   	github.com/knative/eventing-sources/pkg/apis/sources	[no test files]
ok  	github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1	10.448s	coverage: 19.7% of statements
?   	github.com/knative/eventing-sources/pkg/client/clientset/versioned	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/clientset/versioned/fake	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/clientset/versioned/scheme	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/clientset/versioned/typed/sources/v1alpha1	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/clientset/versioned/typed/sources/v1alpha1/fake	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/informers/externalversions	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/informers/externalversions/internalinterfaces	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/informers/externalversions/sources	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/informers/externalversions/sources/v1alpha1	[no test files]
?   	github.com/knative/eventing-sources/pkg/client/listers/sources/v1alpha1	[no test files]
?   	github.com/knative/eventing-sources/pkg/controller	[no test files]
ok  	github.com/knative/eventing-sources/pkg/controller/containersource	0.483s	coverage: 77.1% of statements
?   	github.com/knative/eventing-sources/pkg/controller/containersource/resources	[no test files]
ok  	github.com/knative/eventing-sources/pkg/controller/edgesource	12.301s	coverage: 67.6% of statements
?   	github.com/knative/eventing-sources/pkg/controller/sdk	[no test files]
?   	github.com/knative/eventing-sources/pkg/controller/testing	[no test files]
?   	github.com/knative/eventing-sources/cmd/heartbeats	[no test files]
?   	github.com/knative/eventing-sources/cmd/manager	[no test files]
```

I can see what kubebuilder has done for me:

```
⦿ git status
On branch edgesource
Your branch is ahead of 'origin/edgesource' by 1 commit.
  (use "git push" to publish your local commits)

Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git checkout -- <file>..." to discard changes in working directory)

	modified:   Gopkg.lock
	modified:   config/default.yaml
	modified:   config/rbac/rbac_role.yaml
	modified:   pkg/apis/sources/v1alpha1/zz_generated.deepcopy.go
	modified:   pkg/client/clientset/versioned/typed/sources/v1alpha1/fake/fake_sources_client.go
	modified:   pkg/client/clientset/versioned/typed/sources/v1alpha1/generated_expansion.go
	modified:   pkg/client/clientset/versioned/typed/sources/v1alpha1/sources_client.go
	modified:   pkg/client/informers/externalversions/generic.go
	modified:   pkg/client/informers/externalversions/sources/v1alpha1/interface.go
	modified:   pkg/client/listers/sources/v1alpha1/expansion_generated.go

Untracked files:
  (use "git add <file>..." to include in what will be committed)

	config/crds/sources_v1alpha1_edgesource.yaml
	config/samples/
	pkg/apis/sources/v1alpha1/edgesource_types.go
	pkg/apis/sources/v1alpha1/edgesource_types_test.go
	pkg/apis/sources/v1alpha1/v1alpha1_suite_test.go
	pkg/client/clientset/versioned/typed/sources/v1alpha1/edgesource.go
	pkg/client/clientset/versioned/typed/sources/v1alpha1/fake/fake_edgesource.go
	pkg/client/informers/externalversions/sources/v1alpha1/edgesource.go
	pkg/client/listers/sources/v1alpha1/edgesource.go
	pkg/controller/add_edgesource.go
	pkg/controller/edgesource/
```

### Add Sink and Conditions

We want to add knative style status.conditions to the Status object, and add
sink to the Spec object. While we are here, add the Mark and GetConditions
methods:

```
⦿ git diff pkg/apis/sources/v1alpha1/edgesource_types.go
diff --git a/pkg/apis/sources/v1alpha1/edgesource_types.go b/pkg/apis/sources/v1alpha1/edgesource_types.go
index 9cd20044..0e2e7343 100644
--- a/pkg/apis/sources/v1alpha1/edgesource_types.go
+++ b/pkg/apis/sources/v1alpha1/edgesource_types.go
@@ -17,22 +17,70 @@ limitations under the License.
 package v1alpha1

 import (
+       duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
+       corev1 "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
 )

-// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
-// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
-
 // EdgeSourceSpec defines the desired state of EdgeSource
 type EdgeSourceSpec struct {
-       // INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
-       // Important: Run "make" to regenerate code after modifying this file
+       // Sink is a reference to an object that will resolve to a domain name to use as the sink.
+       // +optional
+       Sink *corev1.ObjectReference `json:"sink,omitempty"`
 }

+const (
+       // ContainerSourceConditionReady has status True when the ContainerSource is ready to send events.
+       EdgeConditionReady = duckv1alpha1.ConditionReady
+
+       // EdgeConditionSinkProvided has status True when the EdgeSource has been configured with a sink target.
+       EdgeConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"
+)
+
+var edgeCondSet = duckv1alpha1.NewLivingConditionSet(
+       EdgeConditionSinkProvided)
+
 // EdgeSourceStatus defines the observed state of EdgeSource
 type EdgeSourceStatus struct {
-       // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
-       // Important: Run "make" to regenerate code after modifying this file
+       // Conditions holds the state of a source at a point in time.
+       // +optional
+       // +patchMergeKey=type
+       // +patchStrategy=merge
+       Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
+
+       // SinkURI is the current active sink URI that has been configured for the ContainerSource.
+       // +optional
+       SinkURI string `json:"sinkUri,omitempty"`
+}
+
+// GetCondition returns the condition currently associated with the given type, or nil.
+func (s *EdgeSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
+       return edgeCondSet.Manage(s).GetCondition(t)
+}
+
+// IsReady returns true if the resource is ready overall.
+func (s *EdgeSourceStatus) IsReady() bool {
+       return edgeCondSet.Manage(s).IsHappy()
+}
+
+// InitializeConditions sets relevant unset conditions to Unknown state.
+func (s *EdgeSourceStatus) InitializeConditions() {
+       edgeCondSet.Manage(s).InitializeConditions()
+}
+
+// MarSink sets the condition that the source has a sink configured.
+func (s *EdgeSourceStatus) MarkSink(uri string) {
+       s.SinkURI = uri
+       if len(uri) > 0 {
+               containerCondSet.Manage(s).MarkTrue(EdgeConditionSinkProvided)
+       } else {
+               containerCondSet.Manage(s).MarkUnknown(EdgeConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
+       }
+}
+
+// MarkNoSink sets the condition that the source does not have a sink configured.
+func (s *EdgeSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
+       containerCondSet.Manage(s).MarkFalse(ContainerConditionSinkProvided, reason, messageFormat, messageA...)
 }

 // +genclient
```

#### Issue: lastTransitionTime fails to validate in the OpenAPI webhook

Conditions uses a celever time wrapper for `lastTransitionTime`. The generators
give:

```
⦿ cat config/crds/sources_v1alpha1_edgesource.yaml
...
        status:
          properties:
            conditions:
              items:
                properties:
                  lastTransitionTime:
                    type: object
                  message:
                    type: string
                  reason:
                    type: string
                  status:
                    type: string
                  type:
                    type: string
...
```

So we need to patch the CRD that `kubebuilder` made using `kustomize`:

```
⦿ cat config/default/edgesource_conditions_patch.yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: edgesources.sources.eventing.knative.dev
spec:
  validation:
    openAPIV3Schema:
      properties:
        status:
          properties:
            conditions:
              items:
                properties:
                  lastTransitionTime:
                    type: string
```

This will overwrite the default CRD that is generated to work with our custom
type.

And add in the CRD to the default `kustomize` run:

```
⦿ git diff
diff --git a/config/default/kustomization.yaml b/config/default/kustomization.yaml
index 8073e340..b2d68be7 100644
--- a/config/default/kustomization.yaml
+++ b/config/default/kustomization.yaml
@@ -8,6 +8,7 @@ namespace: knative-sources
 # markers ("---").
 resources:
 - ../crds/sources_v1alpha1_containersource.yaml
+- ../crds/sources_v1alpha1_edgesource.yaml
 - ../rbac/rbac_role.yaml
 - ../rbac/rbac_role_binding.yaml
 - ../manager/manager.yaml
@@ -15,3 +16,4 @@ resources:
 patches:
 - manager_image_patch.yaml
 - containersource_conditions_patch.yaml
+- edgesource_conditions_patch.yaml
```

### Use the sdk controller

For most cases, the sdk wrapper around controller runtime's reconciler
expectations handles a lot of the work around posting updates to status
and watching for dependent resources. So I switch it out by copying an existing
integration with and making edits...

Remember to update the package name.

see the [PR](https://github.com/knative/eventing-sources/pull/35) for these
details.




