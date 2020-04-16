/*
Copyright 2020 The Knative Authors

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
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

type defaultKafkaTestArgs struct {
	Name       string
	Initial    KafkaSource
	Expected   string
	AssertFunc func(t *testing.T, ks KafkaSource, expected string)
}

func TestSetDefaults(t *testing.T) {
	assertUUID := func(t *testing.T, ks KafkaSource, expected string) {
		consumerGroup := strings.Split(ks.Spec.ConsumerGroup, uuidPrefix)
		_, err := uuid.Parse(consumerGroup[len(consumerGroup)-1])
		if err != nil {
			t.Fatalf("Error Parsing UUID value: %w", err)
		}
	}
	assertGivenGroup := func(t *testing.T, ks KafkaSource, expected string) {
		if diff := cmp.Diff(ks.Spec.ConsumerGroup, expected); diff != "" {
			t.Fatalf("Unexpected consumerGroup Set (-want, +got): %s", diff)
		}
	}
	testCases := []defaultKafkaTestArgs{
		{
			Name:       "nil spec",
			Initial:    KafkaSource{},
			AssertFunc: assertUUID,
		},
		{
			Name: "Set consumerGroup",
			Initial: KafkaSource{
				Spec: KafkaSourceSpec{
					ConsumerGroup: "foo",
				},
			},
			Expected:   "foo",
			AssertFunc: assertGivenGroup,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Initial.SetDefaults(context.TODO())
			if tc.AssertFunc != nil {
				tc.AssertFunc(t, tc.Initial, tc.Expected)
			}
		})
	}
}
