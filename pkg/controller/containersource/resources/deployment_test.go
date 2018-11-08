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

package resources

import (
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestMakeDeployment_sink(t *testing.T) {
	args := &ContainerArguments{
		Name:      "test-name",
		Namespace: "test-namespace",
		Image:     "test-image",
		Args:      []string{"--test1=args1", "--test2=args2"},
		Env: []corev1.EnvVar{{
			Name:  "test1",
			Value: "arg1",
		}, {
			Name: "test2",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "test2-secret",
				},
			},
		}},
		ServiceAccountName: "test-service-account",
		SinkInArgs:         false,
		Sink:               "test-sink",
	}

	want := `{"kind":"Deployment","apiVersion":"apps/v1","metadata":{"generateName":"test-name-","namespace":"test-namespace","creationTimestamp":null},"spec":{"selector":{"matchLabels":{"source":"test-name"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"source":"test-name"},"annotations":{"sidecar.istio.io/inject":"true"}},"spec":{"containers":[{"name":"source","image":"test-image","args":["--test1=args1","--test2=args2","--sink=test-sink"],"env":[{"name":"test1","value":"arg1"},{"name":"test2","valueFrom":{"secretKeyRef":{"key":"test2-secret"}}},{"name":"SINK","value":"test-sink"}],"resources":{},"imagePullPolicy":"IfNotPresent"}],"serviceAccountName":"test-service-account"}},"strategy":{}},"status":{}}`

	deploy := MakeDeployment(nil, args)
	got, err := json.Marshal(deploy)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}

func TestMakeDeployment_sinkinargs(t *testing.T) {
	args := &ContainerArguments{
		Name:      "test-name",
		Namespace: "test-namespace",
		Image:     "test-image",
		Args:      []string{"--test1=args1", "--test2=args2", "--sink=test-sink"},
		Env: []corev1.EnvVar{{
			Name:  "test1",
			Value: "arg1",
		}, {
			Name: "test2",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "test2-secret",
				},
			},
		}},
		ServiceAccountName: "test-service-account",
		SinkInArgs:         true,
	}

	want := `{"kind":"Deployment","apiVersion":"apps/v1","metadata":{"generateName":"test-name-","namespace":"test-namespace","creationTimestamp":null},"spec":{"selector":{"matchLabels":{"source":"test-name"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"source":"test-name"},"annotations":{"sidecar.istio.io/inject":"true"}},"spec":{"containers":[{"name":"source","image":"test-image","args":["--test1=args1","--test2=args2","--sink=test-sink"],"env":[{"name":"test1","value":"arg1"},{"name":"test2","valueFrom":{"secretKeyRef":{"key":"test2-secret"}}},{"name":"SINK"}],"resources":{},"imagePullPolicy":"IfNotPresent"}],"serviceAccountName":"test-service-account"}},"strategy":{}},"status":{}}`

	deploy := MakeDeployment(nil, args)
	got, err := json.Marshal(deploy)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
