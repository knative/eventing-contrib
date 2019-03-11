package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"github.com/google/uuid"
	"github.com/knative/eventing-sources/contrib/kafka/eventsource/pkg/eventsourceconfig"
	"github.com/knative/pkg/cloudevents"
)

func main() {

	eventsourceconfig := eventsourceconfig.GetConfig()
	log.Printf("BOOTSTRAP_SERVERS: %s", eventsourceconfig.BootStrapServers)

	//Config
	config := cluster.NewConfig()
	config.Consumer.Return.Errors = true
	config.Group.Return.Notifications = true
	config.Net.MaxOpenRequests = int(eventsourceconfig.NetMaxOpenRequests)
	config.Net.KeepAlive = time.Duration(eventsourceconfig.NetKeepAlive)
	config.Net.SASL.Enable = eventsourceconfig.NetSaslEnable
	config.Net.SASL.Handshake = eventsourceconfig.NetSaslHandshake
	config.Net.SASL.User = eventsourceconfig.NetSaslUser
	config.Net.SASL.Password = eventsourceconfig.NetSaslPassword
	config.Consumer.MaxWaitTime = time.Duration(eventsourceconfig.ConsumerMaxWaitTime)
	config.Consumer.MaxProcessingTime = time.Duration(eventsourceconfig.ConsumerMaxProcessingTime)
	config.Consumer.Offsets.CommitInterval = time.Duration(eventsourceconfig.ConsumerOffsetsCommitInterval)
	config.Consumer.Offsets.Retention = time.Duration(eventsourceconfig.ConsumerOffsetsRetention)
	config.Consumer.Offsets.Retry.Max = int(eventsourceconfig.ConsumerOffsetsRetryMax)
	config.ChannelBufferSize = int(eventsourceconfig.ChannelBufferSize)
	config.Group.Session.Timeout = time.Duration(eventsourceconfig.GroupSessionTimeout)

	kafkaversion, err := sarama.ParseKafkaVersion(eventsourceconfig.KafkaVersion)
	if err != nil {
		log.Println("Unsupported Kafka Version. Should be in format 2.0.0")
		log.Printf("Defaulting to minimum supported version: %s", kafkaversion)
	}
	config.Version = kafkaversion
	log.Printf("Setting Kafka version to: %s", config.Version)

	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	if eventsourceconfig.ConsumerOffsetsInitial == "OffsetOldest" {
		log.Println("Setting to offsetOldest")
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	config.Group.PartitionStrategy = cluster.StrategyRange
	if eventsourceconfig.GroupPartitionStrategy == "RoundRobin" {
		config.Group.PartitionStrategy = cluster.StrategyRoundRobin
	}

	// init consumer
	brokers := []string{eventsourceconfig.BootStrapServers}
	topics := []string{eventsourceconfig.KafkaTopic}

	consumerGroupID := eventsourceconfig.ConsumerGroupID
	if consumerGroupID == "" {
		consumerGroupID = uuid.New().String()
	}
	log.Printf("Setting ConsumerGroupID to: %s", consumerGroupID)

	config.Consumer.Return.Errors = false

	consumer, err := cluster.NewConsumer(brokers, consumerGroupID, topics, config)
	if err != nil {
		panic(err)
	}
	defer consumer.Close()

	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// consume notifications
	go func() {
		for ntf := range consumer.Notifications() {
			log.Printf("Rebalanced: %+v\n", ntf)
		}
	}()

	// consume messages, watch signals
	for {
		select {
		case msg, ok := <-consumer.Messages():
			if ok {
				// fmt.Fprintf(os.Stdout, "%s/%d/%d\t%s\t%s\n", msg.Topic, msg.Partition, msg.Offset, msg.Key, msg.Value)
				log.Printf("Received %s", msg.Value)

				var payload interface{}

				err := json.Unmarshal(msg.Value, &payload)
				if err != nil {
					log.Printf("Unable to unmarshal message value: %s", msg.Value)
					log.Printf("Error: %s", err)

					// use the byte array as value
					payload = msg.Value

				}

				// post
				go func() {
					posterr := postMessage(eventsourceconfig.KafkaTopic, eventsourceconfig.Target, msg, payload)
					if posterr == nil {
						consumer.MarkOffset(msg, "") // mark message as processed
					} else {
						log.Printf("Error posting message: %s", err)
					}
				}()

			}
		case <-signals:
			return
		}
	}
}

// Creates a CloudEvent Context for a given Kafka ConsumerMessage.
func cloudEventsContext(key []byte, timestamp time.Time, partition int32, offset int64, topic string, contentType string) *cloudevents.EventContext {

	extensions := map[string]interface{}{
		"Kafka-Key": string(key),
	}

	return &cloudevents.EventContext{
		// Events are themselves object and have a unique UUID. Could also have used the UID
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          "dev.knative.eventing.kafka",
		EventID:            "partition:" + strconv.Itoa(int(partition)) + "/offset:" + strconv.FormatInt(offset, 10),
		Source:             topic,
		EventTime:          timestamp,
		Extensions:         extensions,
		ContentType:        contentType,
	}
}

func postMessage(topic string, target string, message *sarama.ConsumerMessage, value interface{}) error {
	ctx := cloudEventsContext(message.Key, message.Timestamp, message.Partition, message.Offset, topic, "application/json")

	log.Printf("Posting to %q", target)
	// Explicitly using Binary encoding so that Istio, et. al. can better inspect
	// event metadata.
	req, err := cloudevents.Binary.NewRequest(target, value, *ctx)
	if err != nil {
		log.Printf("Failed to create http request: %s", err)
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to do POST: %v", err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("response Status: %s", resp.Status)

	return nil
}
