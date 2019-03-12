package main

import (
	"context"
	"flag"
	"log"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/google/uuid"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"github.com/knative/pkg/signals"
)

var (
	sink      string
	topic     string
	groupID   string
	bootstrap string
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to send messages to")
	flag.StringVar(&topic, "topic", "", "the topic to read messages from")
	flag.StringVar(&groupID, "groupId", uuid.New().String(), "the consumer group")
	flag.StringVar(&bootstrap, "bootstrap", "localhost:9092", "Apache Kafka bootstrap servers")
}

func main() {

	flag.Parse()

	kafkaConfig := cluster.NewConfig()
	kafkaConfig.Group.Mode = cluster.ConsumerModePartitions
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	c, err := kncloudevents.NewDefaultClient(sink)
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	// init consumer
	brokers := []string{bootstrap}
	topics := []string{topic}

	consumer, err := cluster.NewConsumer(brokers, groupID, topics, kafkaConfig)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()

	// trap SIGINT to trigger a shutdown.
	stopCh := signals.SetupSignalHandler()

	// consume messages, watch signals
	for {
		select {
		case part, ok := <-consumer.Partitions():
			if !ok {
				return
			}

			// start a separate goroutine to consume messages
			go func(pc cluster.PartitionConsumer) {
				for msg := range pc.Messages() {
					log.Printf("Received %s", msg.Value)
					go post(msg.Value, c)
					consumer.MarkOffset(msg, "") // mark message as processed
				}
			}(part)
		case <-stopCh:
			return
		}
	}
}

func post(data interface{}, c client.Client) {
	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type:   "kafka-event",
			Source: *types.ParseURLRef(topic),
		}.AsV02(),
		Data: data,
	}
	if _, err := c.Send(context.TODO(), event); err != nil {
		log.Printf("sending event to channel failed: %v", err)
	}
}
