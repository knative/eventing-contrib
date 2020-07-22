package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/Shopify/sarama"
	cloudeventskafka "github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
)

type envConfig struct {
	// Port on which to listen for cloudevents
	Port int `envconfig:"PORT" default:"8080"`

	// KafkaServer URL to connect to the Kafka server.
	KafkaServer string `envconfig:"KAFKA_SERVER" required:"true"`

	// Subject is the nats subject to publish cloudevents on.
	Topic string `envconfig:"KAFKA_TOPIC" required:"true"`
}

const (
	count = 10
)

func main() {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_0_0_0

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Printf("[ERROR] Failed to process envirnment variables: %s", err)
		os.Exit(1)
	}

	ctx := context.Background()

	log.Printf("Using HTTP PORT=%d", env.Port)
	httpProtocol, err := cloudeventshttp.New(cloudeventshttp.WithPort(env.Port))
	if err != nil {
		log.Fatalf("failed to create http protocol: %s", err.Error())
	}

	log.Printf("Sinking to KAFKA_SERVER=%s KAFKA_TOPIC=%s", env.KafkaServer, env.Topic)
	kafkaProtocol, err := cloudeventskafka.NewSender([]string{env.KafkaServer}, saramaConfig, env.Topic)
	if err != nil {
		log.Fatalf("failed to create Kafka protcol, %s", err.Error())
	}

	defer kafkaProtocol.Close(ctx)

	// Pipe all messages incoming from httpProtocol to kafkaProtocol
	go func() {
		for {
			log.Printf("Ready to receive")
			// Blocking call to wait for new messages from httpProtocol
			message, err := httpProtocol.Receive(ctx)
			log.Printf("Received message")
			if err != nil {
				if err == io.EOF {
					return // Context closed and/or receiver closed
				}
				log.Printf("Error while receiving a message: %s", err.Error())
			}
			log.Printf("Sending message to Kafka")
			err = kafkaProtocol.Send(ctx, message)
			if err != nil {
				log.Printf("Error while forwarding the message: %s", err.Error())
			}
		}
	}()

	// Start the HTTP Server invoking OpenInbound()
	go func() {
		if err := httpProtocol.OpenInbound(ctx); err != nil {
			log.Printf("failed to StartHTTPReceiver, %v", err)
		}
	}()

	<-ctx.Done()
}
