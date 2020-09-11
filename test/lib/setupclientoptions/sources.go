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

package setupclientoptions

import (
	"context"

	"github.com/google/uuid"

	"knative.dev/eventing-contrib/test/e2e/helpers"
	contribtestlib "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// KafkaSourceV1B1ClientSetupOption returns a ClientSetupOption that can be used
// to create a new KafkaSource. It creates a ServiceAccount, a Role, a
// RoleBinding, a RecordEvents pod and an ApiServerSource object with the event
// mode and RecordEvent pod as its sink.
func KafkaSourceV1B1ClientSetupOption(name string, kafkaClusterName string, kafkaClusterNamespace string, kafkaBootstrapUrl string, recordEventsPodName string) testlib.SetupClientOption {
	return func(client *testlib.Client) {

		var (
			kafkaTopicName = uuid.New().String()
			consumerGroup  = uuid.New().String()
		)

		helpers.MustCreateTopic(client, kafkaClusterName, kafkaClusterNamespace, kafkaTopicName)

		recordevents.StartEventRecordOrFail(context.Background(), client, recordEventsPodName)

		contribtestlib.CreateKafkaSourceV1Beta1OrFail(client, contribresources.KafkaSourceV1Beta1(
			kafkaBootstrapUrl,
			kafkaTopicName,
			resources.ServiceRef(recordEventsPodName),
			contribresources.WithNameV1Beta1(name),
			contribresources.WithConsumerGroupV1Beta1(consumerGroup),
		))

		client.WaitForAllTestResourcesReadyOrFail(context.Background())
	}
}
