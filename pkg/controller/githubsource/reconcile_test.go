/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
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
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

var (
	trueVal = true
)

const (
	image            = "github.com/knative/test/image"
	gitHubSourceName = "testgithubsource"
	testNS           = "testnamespace"
	gitHubSourceUID  = "2b2219e2-ce67-11e8-b3a3-42010a8a00af"

	sinkableDNS = "sinkable.sink.svc.cluster.local"
	sinkableURI = "http://sinkable.sink.svc.cluster.local/"

	sinkableName       = "testsink"
	sinkableKind       = "Sink"
	sinkableAPIVersion = "duck.knative.dev/v1alpha1"

	unsinkableName       = "testunsinkable"
	unsinkableKind       = "KResource"
	unsinkableAPIVersion = "duck.knative.dev/v1alpha1"

	secretName     = "testsecret"
	accessTokenKey = "accessToken"
	secretTokenKey = "secretToken"

	serviceName = gitHubSourceName + "-abc"
	serviceDNS  = serviceName + "." + testNS + ".svc.cluster.local"

	webhookData = "webhookCreatorData"
)

// Adds the list of known types to Scheme.
func duckAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		duckv1alpha1.SchemeGroupVersion,
		&duckv1alpha1.SinkList{},
	)
	metav1.AddToGroupVersion(scheme, duckv1alpha1.SchemeGroupVersion)
	return nil
}

func init() {
	// Add types to scheme
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
	duckAddKnownTypes(scheme.Scheme)
	servingv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name:         "non existent key",
		Reconciles:   &sourcesv1alpha1.GitHubSource{},
		ReconcileKey: "non-existent-test-ns/non-existent-test-key",
		WantErr:      false,
	}, {
		Name:       "valid githubsource, but sink does not exist",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			getGitHubSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		WantErrMsg:   `sinks.duck.knative.dev "testsink" not found`,
	}, {
		Name:       "valid githubsource, but sink is not sinkable",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			getGitHubSourceUnsinkable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// unsinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": unsinkableAPIVersion,
					"kind":       unsinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      unsinkableName,
					},
				},
			},
		},
		WantErrMsg: "sink does not contain sinkable",
	}, {
		Name:       "valid githubsource, sink is sinkable",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			getGitHubSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Status.InitializeConditions()
				s.Status.MarkSink(sinkableURI)
				s.Status.MarkValid()
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid githubsource, sink is sinkable but sink is nil",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			getGitHubSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}(nil),
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("NotFound", "")
				s.Status.MarkValid()
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink does not contain sinkable`,
	}, {
		Name:       "invalid githubsource, sink is nil",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.Sink = nil
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}(nil),
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.Sink = nil
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("NotFound", "")
				s.Status.MarkValid()
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink ref is nil`,
	}, {
		Name:       "invalid githubsource, repository is empty",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.Repository = ""
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.Repository = ""
				s.Status.InitializeConditions()
				s.Status.MarkNotValid("RepositoryMissing", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `repository is empty`,
	}, {
		Name:       "invalid githubsource, eventType is empty",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.EventType = ""
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.EventType = ""
				s.Status.InitializeConditions()
				s.Status.MarkNotValid("EventTypeMissing", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `event type is empty`,
	}, {
		Name:       "invalid githubsource, access token ref is nil",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.AccessToken.SecretKeyRef = nil
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.AccessToken.SecretKeyRef = nil
				s.Status.InitializeConditions()
				s.Status.MarkNotValid("AccessTokenMissing", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `access token ref is nil`,
	}, {
		Name:       "invalid githubsource, secret token ref is nil",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.SecretToken.SecretKeyRef = nil
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.Spec.SecretToken.SecretKeyRef = nil
				s.Status.InitializeConditions()
				s.Status.MarkNotValid("SecretTokenMissing", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `secret token ref is nil`,
	}, {
		Name:       "valid githubsource, webhook created",
		Reconciles: &sourcesv1alpha1.GitHubSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.UID = gitHubSourceUID
				return s
			}(),
			// service resource
			func() runtime.Object {
				svc := &servingv1alpha1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNS,
						Name:      serviceName,
					},
					Status: servingv1alpha1.ServiceStatus{
						Conditions: duckv1alpha1.Conditions{{
							Type:   servingv1alpha1.ServiceConditionRoutesReady,
							Status: corev1.ConditionTrue,
						}},
						Domain: serviceDNS,
					},
				}
				svc.SetOwnerReferences(getOwnerReferences())
				return svc
			}(),
			func() runtime.Object {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: testNS,
						Name:      secretName,
					},
				}
				return secret
			}(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				hookID: "randomhookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitHubSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitHubSource()
				s.UID = gitHubSourceUID
				s.Status.InitializeConditions()
				s.Status.MarkSink(sinkableURI)
				s.Status.MarkValid()
				s.Status.WebhookIDKey = "randomhookid"
				return s
			}(),
		},
		IgnoreTimes: true,
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()

		r := &reconciler{
			dynamicClient:  dc,
			scheme:         tc.Scheme,
			recorder:       recorder,
			webhookCreator: createWebhookCreator(tc.OtherTestData[webhookData]),
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getGitHubSource() *sourcesv1alpha1.GitHubSource {
	obj := &sourcesv1alpha1.GitHubSource{
		TypeMeta:   gitHubSourceType(),
		ObjectMeta: om(testNS, gitHubSourceName),
		Spec: sourcesv1alpha1.GitHubSourceSpec{
			Repository: "foo/bar",
			EventType:  "dev.knative.source.github.pullrequest",
			AccessToken: sourcesv1alpha1.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: accessTokenKey,
				},
			},
			SecretToken: sourcesv1alpha1.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: secretTokenKey,
				},
			},
			Sink: &corev1.ObjectReference{
				Name:       sinkableName,
				Kind:       sinkableKind,
				APIVersion: sinkableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getGitHubSourceUnsinkable() *sourcesv1alpha1.GitHubSource {
	obj := &sourcesv1alpha1.GitHubSource{
		TypeMeta:   gitHubSourceType(),
		ObjectMeta: om(testNS, gitHubSourceName),
		Spec: sourcesv1alpha1.GitHubSourceSpec{
			Repository: "foo/bar",
			EventType:  "dev.knative.source.github.pullrequest",
			AccessToken: sourcesv1alpha1.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: accessTokenKey,
				},
			},
			SecretToken: sourcesv1alpha1.SecretValueFromSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: secretTokenKey,
				},
			},
			Sink: &corev1.ObjectReference{
				Name:       unsinkableName,
				Kind:       unsinkableKind,
				APIVersion: unsinkableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func gitHubSourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "GitHubSource",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "GitHubSource",
		Name:       gitHubSourceName,
		Controller: &trueVal,
		UID:        gitHubSourceUID,
	}}
}

type webhookCreatorData struct {
	clientCreateErr error
	hookID          string
}

func createWebhookCreator(value interface{}) webhookCreator {
	var data webhookCreatorData
	var ok bool
	if data, ok = value.(webhookCreatorData); !ok {
		data = webhookCreatorData{}
	}
	if data.clientCreateErr != nil {
		return func(_ context.Context, _ *webhookCreatorOptions) (string, error) {
			return "", data.clientCreateErr
		}
	}
	return func(_ context.Context, _ *webhookCreatorOptions) (string, error) {
		return data.hookID, nil
	}
}

// Direct Unit tests.

func TestObjectNotGitHubSource(t *testing.T) {
	r := reconciler{}
	obj := &corev1.ObjectReference{
		Name:       unsinkableName,
		Kind:       unsinkableKind,
		APIVersion: unsinkableAPIVersion,
	}

	got, gotErr := r.Reconcile(context.TODO(), obj)
	var want runtime.Object = obj
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected returned object (-want, +got) = %v", diff)
	}
	var wantErr error
	if diff := cmp.Diff(wantErr, gotErr); diff != "" {
		t.Errorf("unexpected returned error (-want, +got) = %v", diff)
	}
}

func TestObjectHasDeleteTimestamp(t *testing.T) {
	r := reconciler{}
	obj := getGitHubSource()

	now := metav1.Now()
	obj.DeletionTimestamp = &now
	got, gotErr := r.Reconcile(context.TODO(), obj)
	var want runtime.Object = obj
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected returned object (-want, +got) = %v", diff)
	}
	var wantErr error
	if diff := cmp.Diff(wantErr, gotErr); diff != "" {
		t.Errorf("unexpected returned error (-want, +got) = %v", diff)
	}
}

func TestInjectConfig(t *testing.T) {
	r := reconciler{}

	r.InjectConfig(&rest.Config{})

	if r.dynamicClient == nil {
		t.Errorf("dynamicClient was nil but expected non nil")
	}
}
