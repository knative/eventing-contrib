package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"go.uber.org/zap"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	"github.com/knative/eventing/pkg/event"
	"golang.org/x/net/context"
)

const (
	// Environment variable containing project id
	envProject = "GCPPUBSUB_PROJECT"

	// Sink for messages.
	envSinkURI = "SINK_URI"

	// envTopic is the name of the environment variable that contains the GCP PubSub Topic being
	// subscribed to's name. In the form that is unique within the project. E.g. 'laconia', not
	// 'projects/my-gcp-project/topics/laconia'.
	envTopic = "GCPPUBSUB_TOPIC"

	// Name of the subscription to use
	envSubscription = "GCPPUBSUB_SUBSCRIPTION_ID"
)

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	projectID := getRequiredEnv(envProject)
	topicID := getRequiredEnv(envTopic)
	sinkURI := getRequiredEnv(envSinkURI)
	subscriptionID := getRequiredEnv(envSubscription)

	logger.Info("Starting.", zap.String("projectID", projectID), zap.String("topicID", topicID), zap.String("subscriptionID", subscriptionID), zap.String("sinkURI", sinkURI))

	ctx := context.Background()
	source := fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", projectID, topicID)

	// Creates a client.
	// TODO: Support additional ways of specifying the credentials for creating.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		logger.Fatal("Failed to create client: %v", zap.Error(err))
	}

	sub := client.Subscription(subscriptionID)

	err = sub.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		logger.Debug("Received message", zap.Any("messageData", m.Data))
		err = postMessage(sinkURI, source, m, logger)
		if err != nil {
			logger.Error("Failed to post message", zap.Error(err))
			m.Nack()
		} else {
			m.Ack()
		}
	})
	if err != nil {
		logger.Fatal("Failed to create receive function", zap.Error(err))
	}
}

func getRequiredEnv(envKey string) string {
	if val, defined := os.LookupEnv(envKey); defined {
		return val
	}
	log.Fatalf("required environment variable not defined '%s'", envKey)
	// Unreachable.
	return ""
}

func postMessage(sinkURI, source string, m *pubsub.Message, logger *zap.Logger) error {
	ctx := event.EventContext{
		CloudEventsVersion: event.CloudEventsVersion,
		EventType:          "google.pubsub.topic.publish",
		EventID:            m.ID,
		EventTime:          m.PublishTime,
		Source:             source,
	}
	req, err := event.Binary.NewRequest(sinkURI, m, ctx)
	if err != nil {
		logger.Error("Failed to marshal the message.", zap.Error(err), zap.Any("message", m))
		return err
	}

	logger.Debug("Posting message", zap.String("sinkURI", sinkURI))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debug("Response", zap.String("status", resp.Status), zap.ByteString("body", body))
	return nil
}
