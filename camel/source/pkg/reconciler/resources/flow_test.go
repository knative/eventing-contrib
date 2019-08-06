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

func TestCamelFlow(t *testing.T) {
	type TestCase struct {
		Name   string
		Sink   string
		Input  string
		Output string
	}
	testCases := []TestCase{
		{
			Name: "standard-test",
			Input: `
from:
  uri: theuri
  steps:
  - step: step
`,
			Output: `
from:
  uri: theuri
  steps:
  - step: step
  - to:
      uri: knative://endpoint/sink
`,
		},
		{
			Name: "no-steps-test",
			Input: `
from:
  uri: theuri
`,
			Output: `
from:
  uri: theuri
  steps:
  - to:
      uri: knative://endpoint/sink
`,
		},
		{
			Name:  "empty-test",
			Input: ``,
			Output: `
from:
  steps:
  - to:
      uri: knative://endpoint/sink
`,
		},
		{
			Name: "wrong-struct-test",
			Input: `
from:
  steps: hello
`,
			Output: `
from:
  steps:
  - to:
      uri: knative://endpoint/sink
`,
		},
		{
			Name: "complex-scenario-test",
			Input: `
id: 1
from:
  uri: theuri
  another: field
  steps:
    - f1
    - f2:
        to: 2
`,
			Output: `
id: 1
from:
  uri: theuri
  another: field
  steps:
    - f1
    - f2:
        to: 2
    - to:
        uri: knative://endpoint/another-sink
`,
			Sink: "another-sink",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// Test that input is converted into output
			flow, err := UnmarshalCamelFlow(tc.Input)
			if err != nil {
				t.Error(err)
			}
			sink := tc.Sink
			if sink == "" {
				sink = "sink"
			}
			newFlow := AddSinkToCamelFlow(flow, sink)

			eFlow, err := UnmarshalCamelFlow(tc.Output)
			if err != nil {
				t.Error(err)
			}

			if diff := cmp.Diff(eFlow, newFlow); diff != "" {
				t.Errorf("wrong result data (-want, +got) = %v", diff)
			}

			// Testing that marshal and unmarshal work for result data
			eFlow2, err := CopyCamelFlow(eFlow)
			if err != nil {
				t.Error(err)
			}
			if diff := cmp.Diff(eFlow, eFlow2); diff != "" {
				t.Errorf("got different data after marshalling (-want, +got) = %v", diff)
			}
		})
	}

}
