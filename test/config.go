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
package test

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

// ChannelFeatureMap saves the channel-features mapping.
// Each pair means the channel support the list of features.
var ChannelFeatureMap = map[metav1.TypeMeta][]testlib.Feature{
	{
		APIVersion: resources.MessagingAPIVersion,
		Kind:       KafkaChannelKind,
	}: {
		testlib.FeatureBasic,
		testlib.FeatureRedelivery,
		testlib.FeaturePersistence,
	},
	{
		APIVersion: resources.MessagingAPIVersion,
		Kind:       NatssChannelKind,
	}: {
		testlib.FeatureBasic,
		testlib.FeatureRedelivery,
		testlib.FeaturePersistence,
	},
}
