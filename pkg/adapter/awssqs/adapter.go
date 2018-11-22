/*
Copyright 2018 The Knative Authors

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

package awssqs

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/knative/pkg/cloudevents"
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	eventType = "aws.sqs.message"
)

// Adapter implements the AWS SQS adapter to deliver SQS messages from
// an SQS queue to a Sink.
type Adapter struct {

	// Region is the AWS region
	Region string

	// QueueUrl is the AWS SQS URL that we're polling messages from
	QueueUrl string

	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	// CredsFile is the full path of the AWS credentials file
	CredsFile string

	// OnFailedPollWaitSecs determines the interval to wait after a
	// failed poll before making another one
	OnFailedPollWaitSecs time.Duration
}

func (a *Adapter) Start(ctx context.Context) error {

	logger := logging.FromContext(ctx)

	stopCh := signals.SetupSignalHandler()
	cctx, cancel := context.WithCancel(ctx)

	logger.Info("Starting with config: ", zap.Any("adapter", a))

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config:            aws.Config{Region: aws.String(a.Region)},
		SharedConfigFiles: []string{a.CredsFile},
	}))

	q := sqs.New(sess)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case <-stopCh:
				break
			}
			messages, err := poll(cctx, q, a.QueueUrl, 10)
			if err != nil {
				logger.Warn("Failed to poll from SQS queue", zap.Error(err))
				time.Sleep(a.OnFailedPollWaitSecs * time.Second)
				continue
			}
			for _, m := range messages {
				a.receiveMessage(ctx, m, func() {
					_, err = q.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl:      &a.QueueUrl,
						ReceiptHandle: m.ReceiptHandle,
					})
					if err != nil {
						// the only consequence is that the message will
						// get redelivered later, given that SQS is
						// at-least-once delivery. That should be
						// acceptable as "normal operation"
						logger.Error("Failed to delete message", zap.Error(err))
					}
				})
			}
		}
	}()

	wg.Wait()
	cancel()
	logger.Info("Exiting")

	return nil
}

// receiveMessage handles an incoming message from the AWS SQS queue,
// and forwards it to a Sink, calling `ack()` when the forwarding is
// successful.
func (a *Adapter) receiveMessage(ctx context.Context, m *sqs.Message, ack func()) {
	logger := logging.FromContext(ctx)
	logger.Debug("Received message from SQS:", zap.Any("message", m))

	err := a.postMessage(ctx, m)
	if err != nil {
		logger.Error("Failed to post message to Sink", zap.Error(err))
	} else {
		logger.Debug("Message successfully posted to Sink")
		ack()
	}
}

// postMessage sends an SQS event to the SinkURI
func (a *Adapter) postMessage(ctx context.Context, m *sqs.Message) error {
	logger := logging.FromContext(ctx)

	// TODO verify the timestamp conversion
	timestamp, err := strconv.ParseInt(*m.Attributes["SentTimestamp"], 10, 64)
	if err != nil {
		logger.Error("Failed to marshal the message.", zap.Error(err), zap.Any("message", m.Body))
		timestamp = time.Now().UnixNano()
	}

	event := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          eventType,
		EventID:            *m.MessageId,
		EventTime:          time.Unix(timestamp, 0),
		Source:             a.QueueUrl,
	}
	req, err := cloudevents.Binary.NewRequest(a.SinkURI, m, event)
	if err != nil {
		logger.Error("Failed to marshal the message.", zap.Error(err), zap.Any("message", m))
		return err
	}

	logger.Debug("Posting message", zap.String("sinkURI", a.SinkURI))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debug("Response", zap.String("status", resp.Status), zap.ByteString("body", body))

	// If the response is not within the 2xx range, return an error.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("[%d] unexpected response %s", resp.StatusCode, body)
	}

	return nil
}

// poll reads messages from the queue in batches of a given maximum size.
func poll(ctx context.Context, q *sqs.SQS, url string, maxBatchSize int64) ([]*sqs.Message, error) {

	result, err := q.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameSentTimestamp),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl: &url,
		// Maximum size of the batch of messages returned from the poll.
		MaxNumberOfMessages: aws.Int64(maxBatchSize),
		// Controls the maximum time to wait in the poll performed with
		// ReceiveMessageWithContext.  If there are no messages in the
		// given secs, the call times out and returns control to us.
		WaitTimeSeconds: aws.Int64(5),
	})

	if err != nil {
		return []*sqs.Message{}, err
	}

	return result.Messages, nil
}
