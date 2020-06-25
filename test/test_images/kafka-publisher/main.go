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

package main

import (
	"log"

	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
	"knative.dev/eventing-contrib/kafka"
	"knative.dev/pkg/signals"
)

type envConfig struct {
	Topic   string
	Key     string
	Headers map[string]string
	Value   string
}

func main() {
	ctx := signals.NewContext()

	// Read the configuration out of the environment.
	var s envConfig
	err := envconfig.Process("kafka", &s)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("GOT: %v", s)

	// Create a Kafka client from our Binding.
	client, err := kafka.NewProducer(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}
	producer, err := sarama.NewSyncProducerFromClient(client)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Send the message this Job was created to send.
	headers := make([]sarama.RecordHeader, 0, len(s.Headers))
	for k, v := range s.Headers {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic:   s.Topic,
		Key:     sarama.StringEncoder(s.Key),
		Value:   sarama.StringEncoder(s.Value),
		Headers: headers,
	})
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Print("Fin!")
}
