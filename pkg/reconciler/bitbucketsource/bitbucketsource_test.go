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

package bitbucketsource

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
	"k8s.io/client-go/tools/record"
)

var (
	trueVal = true
	now     = metav1.Now().Rfc3339Copy()
)

const (
	image               = "github.com/knative/test/image"
	bitBucketSourceName = "testbitbucketsource"
	testNS              = "testnamespace"
	bitBucketSourceUID  = "2b2219e2-ce67-11e8-b3a3-42010a8a00af"

	addressableDNS = "addressable.sink.svc.cluster.local"
	addressableURI = "http://addressable.sink.svc.cluster.local/"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"

	unaddressableName       = "testunaddressable"
	unaddressableKind       = "KResource"
	unaddressableAPIVersion = "duck.knative.dev/v1alpha1"

	secretName     = "testsecret"
	consumerKey    = "consumerKey"
	consumerSecret = "consumerSecret"

	serviceName = bitBucketSourceName + "-abc"
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
		Reconciles:   &sourcesv1alpha1.BitBucketSource{},
		ReconcileKey: "non-existent-test-ns/non-existent-test-key",
		WantErr:      false,
	}, {
		Name:       "invalid bitbucketsource, consumer key ref does not exist",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSource(),
			getAddressable(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      secretName,
				},
				Data: map[string][]byte{
					consumerSecret: []byte("bar"),
				},
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSecrets(consumerKeyNotFound,
					fmt.Sprintf(`key "%s" not found in secret "%s"`, consumerKey, secretName))
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  fmt.Sprintf(`key "%s" not found in secret "%s"`, consumerKey, secretName),
	}, {
		Name:       "invalid bitbucketsource, consumer secret ref does not exist",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSource(),
			getAddressable(),
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      secretName,
				},
				Data: map[string][]byte{
					consumerKey: []byte("foo"),
				},
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSecrets(consumerSecretNotFound,
					fmt.Sprintf(`key "%s" not found in secret "%s"`, consumerSecret, secretName))
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  fmt.Sprintf(`key "%s" not found in secret "%s"`, consumerSecret, secretName),
	}, {
		Name:       "invalid bitbucketsource, sink does not exist",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSource(),
			getBitBucketSecrets(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		WantErrMsg:   `sinks.duck.knative.dev "testsink" not found`,
	}, {
		Name:       "invalid bitbucketsource, sink is not addressable",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSourceUnaddressable(),
			getBitBucketSecrets(),
			getAddressableNoStatus(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantErrMsg:   `sink "testnamespace/testunaddressable" (duck.knative.dev/v1alpha1, Kind=KResource) does not contain address`,
	}, {
		Name:       "invalid bitbucketsource, service does not have domain",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSource(),
			getBitBucketSecrets(),
			getAddressable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Status.InitializeConditions()
				s.Status.MarkSecrets()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkNoService(svcDomainNotFound, "%s", `domain not found for svc ""`)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "invalid bitbucketsource, sink is empty",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			getBitBucketSource(),
			getBitBucketSecrets(),
			getAddressableNilAddress(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Status.InitializeConditions()
				s.Status.MarkSecrets()
				s.Status.MarkNoSink(sinkNotFound, "sink \"testnamespace/testsink\" (duck.knative.dev/v1alpha1, Kind=Sink) does not contain address")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink "testnamespace/testsink" (duck.knative.dev/v1alpha1, Kind=Sink) does not contain address`,
	}, {
		Name:       "invalid bitbucketsource, sink is nil",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Spec.Sink = nil
				return s
			}(),
			getBitBucketSecrets(),
			getAddressable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.Spec.Sink = nil
				s.Status.InitializeConditions()
				s.Status.MarkSecrets()
				s.Status.MarkNoSink(sinkNotFound, "sink ref is nil")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink ref is nil`,
	}, {
		Name:       "valid bitbucketsource, repo webhook created",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
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
			getBitBucketSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedOwner: "myuser",
				expectedRepo:  "myproject",
				hookUUID:      "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				s.Status.MarkService()
				s.Status.MarkWebHook("repohookid")
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid bitbucketsource, team webhook created",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.Spec.OwnerAndRepository = "myteam"
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
			getBitBucketSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedOwner: "myteam",
				hookUUID:      "teamhookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.Spec.OwnerAndRepository = "myteam"
				s.Status.InitializeConditions()
				s.Status.MarkSink(addressableURI)
				s.Status.MarkSecrets()
				s.Status.MarkService()
				s.Status.MarkWebHook("teamhookid")
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid bitbucketsource, deleted",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = "repohookid"
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
			getBitBucketSecrets(),
			getAddressable(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedOwner: "myuser",
				expectedRepo:  "myproject",
				hookUUID:      "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = ""
				s.Finalizers = nil
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid bitbucketsource, deleted, missing addressable",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = "repohookid"
				return s
			}(),
			getBitBucketSecrets(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedOwner: "myuser",
				expectedRepo:  "myproject",
				hookUUID:      "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = ""
				s.Finalizers = nil
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid bitbucketsource, deleted, missing secret",
		Reconciles: &sourcesv1alpha1.BitBucketSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = "repohookid"
				return s
			}(),
		},
		OtherTestData: map[string]interface{}{
			webhookData: webhookCreatorData{
				expectedOwner: "myuser",
				expectedRepo:  "myproject",
				hookUUID:      "repohookid",
			},
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, bitBucketSourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getBitBucketSource()
				s.UID = bitBucketSourceUID
				s.Status.MarkNoSecrets(consumerKeyNotFound, "%s", fmt.Errorf("secrets %q not found", secretName))
				s.DeletionTimestamp = &now
				s.Status.WebhookUUIDKey = "repohookid"
				s.Finalizers = nil
				return s
			}(),
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

func getBitBucketSource() *sourcesv1alpha1.BitBucketSource {
	obj := &sourcesv1alpha1.BitBucketSource{
		TypeMeta:   bitBucketSourceType(),
		ObjectMeta: om(testNS, bitBucketSourceName),
		Spec: sourcesv1alpha1.BitBucketSourceSpec{
			OwnerAndRepository: "myuser/myproject",
			EventTypes:         []string{"pullrequest:created"},
			ConsumerKey: sourcesv1alpha1.BitBucketConsumerValue{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: consumerKey,
				},
			},
			ConsumerSecret: sourcesv1alpha1.BitBucketConsumerValue{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: consumerSecret,
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

func getBitBucketSourceUnaddressable() *sourcesv1alpha1.BitBucketSource {
	obj := &sourcesv1alpha1.BitBucketSource{
		TypeMeta:   bitBucketSourceType(),
		ObjectMeta: om(testNS, bitBucketSourceName),
		Spec: sourcesv1alpha1.BitBucketSourceSpec{
			OwnerAndRepository: "myuser/myproject",
			EventTypes:         []string{"pullrequest:created"},
			ConsumerKey: sourcesv1alpha1.BitBucketConsumerValue{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: consumerKey,
				},
			},
			ConsumerSecret: sourcesv1alpha1.BitBucketConsumerValue{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: consumerSecret,
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

func bitBucketSourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "BitBucketSource",
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
		Kind:       "BitBucketSource",
		Name:       bitBucketSourceName,
		Controller: &trueVal,
		UID:        bitBucketSourceUID,
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

func getBitBucketSecrets() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      secretName,
		},
		Data: map[string][]byte{
			consumerKey:    []byte("foo"),
			consumerSecret: []byte("bar"),
		},
	}
}

type mockWebhookClient struct {
	data webhookCreatorData
}

func (client mockWebhookClient) Create(ctx context.Context, options *webhookOptions) (string, error) {
	data := client.data
	if data.clientCreateErr != nil {
		return "", data.clientCreateErr
	}
	if data.expectedOwner != options.owner {
		return "", fmt.Errorf(`expected webhook owner of "%s", got "%s"`,
			data.expectedOwner, options.owner)
	}
	if data.expectedRepo != options.repo {
		return "", fmt.Errorf(`expected webhook repository of "%s", got "%s"`,
			data.expectedRepo, options.repo)
	}
	return data.hookUUID, nil
}

func (client mockWebhookClient) Delete(ctx context.Context, options *webhookOptions) error {
	data := client.data
	if data.expectedOwner != options.owner {
		return fmt.Errorf(`expected webhook owner of "%s", got "%s"`,
			data.expectedOwner, options.owner)
	}
	if data.expectedRepo != options.repo {
		return fmt.Errorf(`expected webhook repository of "%s", got "%s"`,
			data.expectedRepo, options.repo)
	}
	if data.hookUUID != options.uuid {
		return fmt.Errorf(`expected webhook UUID of "%s", got "%s"`,
			data.hookUUID, options.uuid)
	}
	return nil
}

type webhookCreatorData struct {
	clientCreateErr error
	expectedOwner   string
	expectedRepo    string
	hookUUID        string
}

func TestObjectNotBitBucketSource(t *testing.T) {
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
