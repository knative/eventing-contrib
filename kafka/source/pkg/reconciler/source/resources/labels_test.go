/*
Copyright 2019 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetLabels(t *testing.T) {

	testLabels := GetLabels("testSourceName")

	wantLabels := map[string]string{
		"eventing.knative.dev/source":     "kafka-source-controller",
		"eventing.knative.dev/SourceName": "testSourceName",
	}

	eq := cmp.Equal(testLabels, wantLabels)
	if !eq {
		t.Fatalf("%v is not equal to %v", testLabels, wantLabels)
	}
}
