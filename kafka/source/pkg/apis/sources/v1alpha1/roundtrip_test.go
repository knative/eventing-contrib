/*
Copyright 2020 The Knative Authors.

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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	pkgfuzzer "knative.dev/pkg/apis/testing/fuzzer"
	"knative.dev/pkg/apis/testing/roundtrip"

	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
)

func TestSourcesRoundTripTypesToJSON(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(AddToScheme(scheme))

	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(
		pkgfuzzer.Funcs,
		v1beta1.FuzzerFuncs,
	)
	roundtrip.ExternalTypesViaJSON(t, scheme, fuzzerFuncs)
}

func TestSourceRoundTripTypesToAlphaHub(t *testing.T) {
	scheme := runtime.NewScheme()

	sb := runtime.SchemeBuilder{
		AddToScheme,
		v1beta1.AddToScheme,
	}

	utilruntime.Must(sb.AddToScheme(scheme))

	hubs := runtime.NewScheme()
	hubs.AddKnownTypes(SchemeGroupVersion,
		&KafkaSource{},
	)

	fuzzerFuncs := fuzzer.MergeFuzzerFuncs(
		pkgfuzzer.Funcs,
		v1beta1.FuzzerFuncs,
	)

	roundtrip.ExternalTypesViaHub(t, scheme, hubs, fuzzerFuncs)
}
