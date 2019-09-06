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
	"knative.dev/eventing/test/common"
)

// ChannelFeatureMap saves the channel-features mapping.
// Each pair means the channel support the list of features.
var ChannelFeatureMap = map[string][]common.Feature{
	KafkaChannelKind: []common.Feature{
		common.FeatureBasic,
		common.FeatureRedelivery,
		common.FeaturePersistence,
	},
	NatssChannelKind: []common.Feature{
		common.FeatureBasic,
		common.FeatureRedelivery,
		common.FeaturePersistence,
	},
}
