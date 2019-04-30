/*
Copyright 2019 The Knative Authors

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

package gitlabsource

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/gitlab/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
)

var (
	trueVal = true
	now     = metav1.Now().Rfc3339Copy()
)

const (
	image            = "github.com/knative/test/image"
	gitLabSourceName = "testgitlabsource"
	testNS           = "testnamespace"
	gitLabSourceUID  = "2b2219e2-ce67-11e8-b3a3-42010a8a00af"

	addressableDNS = "addressable.sink.svc.cluster.local"
	addressableURI = "http://addressable.sink.svc.cluster.local/"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"

	unaddressableName       = "testunaddressable"
	unaddressableKind       = "KResource"
	unaddressableAPIVersion = "duck.knative.dev/v1alpha1"

	secretName     = "testsecret"
	accessTokenKey = "accessToken"
	secretTokenKey = "secretToken"

	serviceName = gitLabSourceName + "-abc"
	serviceDNS  = serviceName + "." + testNS + ".svc.cluster.local"

	webhookData = "webhookCreatorData"
)

func init() {
	// Add types to scheme
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
	servingv1alpha1.AddToScheme(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name:         "non existent key",
		Reconciles:   &sourcesv1alpha1.GitLabSource{},
		ReconcileKey: "non-existent-test-ns/non-existent-test-key",
		WantErr:      false,
	},
	{
		Name:       "valid gitlabsource, but sink does not exist",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSource(),
			getGitLabSecrets(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		WantErrMsg:   `sinks.duck.knative.dev "testsink" not found`,
	},
	{
		Name:       "valid gitlabsource, but sink is not addressable",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSourceUnaddressable(),
			getGitLabSecrets(),
			getAddressableNoStatus(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantErrMsg:   `sink "testnamespace/testunaddressable" (duck.knative.dev/v1alpha1, Kind=KResource) does not contain address`,
	},
	{
		Name:       "valid githubsource, sink is addressable",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSource(),
			getGitLabSecrets(),
			getAddressable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				return s
			}(),
		},
		IgnoreTimes: true,
	},
	{
		Name:       "valid githubsource, sink is addressable but sink is nil",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSource(),
			getGitLabSecrets(),
			getAddressableNilAddress(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("NotFound", "sink \"testnamespace/testsink\" (duck.knative.dev/v1alpha1, Kind=Sink) does not contain address")
				s.Status.MarkSecrets()
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink "testnamespace/testsink" (duck.knative.dev/v1alpha1, Kind=Sink) does not contain address`,
	},
	{
		Name:       "invalid gitlabsource, sink is nil",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Spec.Sink = nil
				return s
			}(),
			getGitLabSecrets(),
			getAddressable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Spec.Sink = nil
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("NotFound", "sink ref is nil")
				s.Status.MarkSecrets()
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink ref is nil`,
	},
	{
		Name:       "valid gitlabsource, repo webhook created",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
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
						Status: duckv1alpha1.Status{
							Conditions: duckv1alpha1.Conditions{{
								Type:   servingv1alpha1.ServiceConditionRoutesReady,
								Status: corev1.ConditionTrue,
							}},
						},
						RouteStatusFields: servingv1alpha1.RouteStatusFields{
							Domain: serviceDNS,
						},
					},
				}
				svc.SetOwnerReferences(getOwnerReferences())
				return svc
			}(),
			getGitLabSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				hookID:          "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				s.Status.WebhookIDKey = "repohookid"
				return s
			}(),
		},
		IgnoreTimes: true,
	},
	{
		Name:       "valid gitlabsource, secure repo webhook created",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Spec.Secure = true
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
						Status: duckv1alpha1.Status{
							Conditions: duckv1alpha1.Conditions{{
								Type:   servingv1alpha1.ServiceConditionRoutesReady,
								Status: corev1.ConditionTrue,
							}},
						},
						RouteStatusFields: servingv1alpha1.RouteStatusFields{
							Domain: serviceDNS,
						},
					},
				}
				svc.SetOwnerReferences(getOwnerReferences())
				return svc
			}(),
			getGitLabSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				expectedSecure:  true,
				hookID:          "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Spec.Secure = true
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				s.Status.WebhookIDKey = "repohookid"
				return s
			}(),
		},
		IgnoreTimes: true,
	},
	{
		Name:       "valid gitlabsource, org webhook created",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Spec.ProjectURL = "http://best1.fyre.ibm.com:81/root/knative-demo"
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
						Status: duckv1alpha1.Status{
							Conditions: duckv1alpha1.Conditions{{
								Type:   servingv1alpha1.ServiceConditionRoutesReady,
								Status: corev1.ConditionTrue,
							}},
						},
						RouteStatusFields: servingv1alpha1.RouteStatusFields{
							Domain: serviceDNS,
						},
					},
				}
				svc.SetOwnerReferences(getOwnerReferences())
				return svc
			}(),
			getGitLabSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				hookID:          "orghookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Spec.ProjectURL = "http://best1.fyre.ibm.com:81/root/knative-demo"
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				s.Status.WebhookIDKey = "orghookid"
				return s
			}(),
		},
		IgnoreTimes: true,
	},
	{
		Name:       "invalid gitlabsource, secret does not exist",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSource(),
			getAddressable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSecrets("AccessTokenNotFound",
					fmt.Sprintf(`secrets "%s" not found`, secretName))
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  fmt.Sprintf(`secrets "%s" not found`, secretName),
	},
	{
		Name:       "invalid gitlabsource, secret key ref does not exist",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			getGitLabSource(),
			getAddressable(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      secretName,
				},
				Data: map[string][]byte{
					accessTokenKey: []byte("foo"),
				},
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSecrets("SecretTokenNotFound",
					fmt.Sprintf(`key "%s" not found in secret "%s"`, secretTokenKey, secretName))
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  fmt.Sprintf(`key "%s" not found in secret "%s"`, secretTokenKey, secretName),
	},
	{
		Name:       "valid gitlabsource, deleted",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = "repohookid"
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
						Status: duckv1alpha1.Status{
							Conditions: duckv1alpha1.Conditions{{
								Type:   servingv1alpha1.ServiceConditionRoutesReady,
								Status: corev1.ConditionTrue,
							}},
						},
						RouteStatusFields: servingv1alpha1.RouteStatusFields{
							Domain: serviceDNS,
						},
					},
				}
				svc.SetOwnerReferences(getOwnerReferences())
				return svc
			}(),
			getGitLabSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				hookID:          "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = ""
				s.Finalizers = nil
				return s
			}(),
		},
		IgnoreTimes: true,
	},
	{
		Name:       "valid gitlabsource, deleted, missing addressable",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = "repohookid"
				return s
			}(),
			getGitLabSecrets(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				hookID:          "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = ""
				s.Finalizers = nil
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid gitlabsource, deleted, missing secret",
		Reconciles: &sourcesv1alpha1.GitLabSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = "repohookid"
				return s
			}(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedProject: "root/knative-demo",
				hookID:          "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, gitLabSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getGitLabSource()
				s.UID = gitLabSourceUID
				s.Status.MarkNoSecrets("AccessTokenNotFound", "%s", fmt.Errorf("secrets %q not found", secretName))
				s.DeletionTimestamp = &now
				s.Status.WebhookIDKey = "repohookid"
				s.Finalizers = nil
				return s
			}(),
			//TODO check for Event
			// Type: Warning
			// Reason: FailedFinalize
			// Message: Could not delete webhook "repohookid": secrets "testsecret" not found
		},
		IgnoreTimes: true,
		WantErrMsg:  fmt.Sprintf("secrets %q not found", secretName),
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()

		var hookData webhookCreatorData
		var ok bool
		if hookData, ok = tc.OtherTestData[webhookData].(webhookCreatorData); !ok {
			hookData = webhookCreatorData{}
		}

		r := &reconciler{
			scheme:   tc.Scheme,
			recorder: recorder,
			webhookClient: &mockWebhookClient{
				data: hookData,
			},
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getGitLabSource() *sourcesv1alpha1.GitLabSource {
	obj := &sourcesv1alpha1.GitLabSource{
		TypeMeta:   gitLabSourceType(),
		ObjectMeta: om(testNS, gitLabSourceName),
		Spec: sourcesv1alpha1.GitLabSourceSpec{
			ProjectURL: "http://best1.fyre.ibm.com:81/root/knative-demo",
			EventTypes: []string{"Push Hook"},
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
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}
	obj.Finalizers = []string{finalizerName}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getGitLabSourceUnaddressable() *sourcesv1alpha1.GitLabSource {
	obj := &sourcesv1alpha1.GitLabSource{
		TypeMeta:   gitLabSourceType(),
		ObjectMeta: om(testNS, gitLabSourceName),
		Spec: sourcesv1alpha1.GitLabSourceSpec{
			ProjectURL: "http://best1.fyre.ibm.com:81/root/knative-demo",
			EventTypes: []string{"Push Hook"},
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
				Name:       unaddressableName,
				Kind:       unaddressableKind,
				APIVersion: unaddressableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func gitLabSourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "GitLabSource",
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
		Kind:       "GitLabSource",
		Name:       gitLabSourceName,
		Controller: &trueVal,
		UID:        gitLabSourceUID,
	}}
}

func getAddressable() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       addressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": addressableDNS,
				},
			},
		},
	}
}

func getAddressableNoStatus() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": unaddressableAPIVersion,
			"kind":       unaddressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      unaddressableName,
			},
		},
	}
}

func getAddressableNilAddress() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       addressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}(nil),
			},
		},
	}
}

func getGitLabSecrets() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      secretName,
		},
		Data: map[string][]byte{
			accessTokenKey: []byte("foo"),
			secretTokenKey: []byte("bar"),
		},
	}
}

type mockWebhookClient struct {
	data webhookCreatorData
}

func (client mockWebhookClient) Create(ctx context.Context, options *projectHookOptions, baseURL string) (string, error) {
	data := client.data
	if data.clientCreateErr != nil {
		return "", data.clientCreateErr
	}
	if data.expectedProject != options.project {
		return "", fmt.Errorf(`expected webhook project of "%s", got "%s"`,
			data.expectedProject, options.project)
	}
	if data.expectedSecure != options.EnableSSLVerification {
		return "", fmt.Errorf(`expected webhook secure of "%v", got "%v"`,
			data.expectedSecure, options.EnableSSLVerification)
	}
	return data.hookID, nil
}

func (client mockWebhookClient) Delete(ctx context.Context, options *projectHookOptions, baseURL string) error {
	data := client.data
	if data.expectedProject != options.project {
		return fmt.Errorf(`expected webhook project of "%s", got "%s"`,
			data.expectedProject, options.project)
	}
	if data.expectedSecure != options.EnableSSLVerification {
		return fmt.Errorf(`expected webhook secure of "%v", got "%v"`,
			data.expectedSecure, options.EnableSSLVerification)
	}
	hookID := options.webhookID
	if data.hookID != hookID {
		return fmt.Errorf(`expected webhook ID of "%s", got "%s"`,
			data.hookID, hookID)
	}
	return nil
}

type webhookCreatorData struct {
	clientCreateErr error
	expectedProject string
	expectedSecure  bool
	hookID          string
}

// Direct Unit tests.

func TestObjectNotGitHubSource(t *testing.T) {
	r := reconciler{}
	obj := &corev1.ObjectReference{
		Name:       unaddressableName,
		Kind:       unaddressableKind,
		APIVersion: unaddressableAPIVersion,
	}

	got := obj.DeepCopy()
	gotErr := r.Reconcile(context.TODO(), got)
	var want runtime.Object = obj
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected returned object (-want, +got) = %v", diff)
	}
	var wantErr error
	if diff := cmp.Diff(wantErr, gotErr); diff != "" {
		t.Errorf("unexpected returned error (-want, +got) = %v", diff)
	}
}
