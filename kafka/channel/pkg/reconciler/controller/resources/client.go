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
	"github.com/Shopify/sarama"
	"knative.dev/eventing-contrib/kafka"
)

func MakeClient(clientID string, tlscfg map[string]string, bootstrapServers []string) (sarama.ClusterAdmin, error) {
	saramaConf := sarama.NewConfig()
	saramaConf.Version = sarama.V2_0_0_0
	saramaConf.ClientID = clientID

	// we have some TLS
	if tlscfg !=nil {
		saramaConf.Net.TLS.Enable = true
		tlsConfig, err := kafka.NewTLSConfig(tlscfg["Usercert"], tlscfg["Userkey"], tlscfg["Cacert"])
		if err != nil {
			return nil, err
		}
		saramaConf.Net.TLS.Config = tlsConfig
	}
	return sarama.NewClusterAdmin(bootstrapServers, saramaConf)
}
