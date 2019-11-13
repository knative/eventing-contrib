package main

import (
	"fmt"
	"math/rand"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/golang/protobuf/ptypes"
	loadastic_common "github.com/slinkydeveloper/loadastic/common"
	"github.com/slinkydeveloper/loadastic/kafka"
	performance_common "knative.dev/eventing/test/common/performance/common"
	"knative.dev/eventing/test/common/performance/sender"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type KafkaLoadGenerator struct {
	loadastic kafka.Loadastic
	sender    kafka.Sender
}

func (k KafkaLoadGenerator) Warmup(pace performance_common.PaceSpec, msgSize uint) {
	k.loadastic.StartSteps(JsonKafkaRequestFactory(msgSize, performance_common.WarmupEventType), paceToStep(pace))
}

func (k KafkaLoadGenerator) RunPace(i int, pace performance_common.PaceSpec, msgSize uint) {
	k.loadastic.StartSteps(JsonKafkaRequestFactory(msgSize, performance_common.MeasureEventType), paceToStep(pace))
}

func (k KafkaLoadGenerator) SendGCEvent() {
	_, _ = k.sender.Send(generatePayloadWithType(performance_common.GCEventType))
}

func (k KafkaLoadGenerator) SendEndEvent() {
	_, _ = k.sender.Send(generatePayloadWithType(performance_common.EndEventType))
}

func NewKafkaLoadGeneratorFactory(bootstrapUrl string, topic string, minWorkers uint64) sender.LoadGeneratorFactory {
	return func(eventSource string, sentCh chan performance_common.EventTimestamp, acceptedCh chan performance_common.EventTimestamp) (sender.LoadGenerator, error) {
		if bootstrapUrl == "" {
			panic("Missing --bootstrap-url flag")
		}

		if topic == "" {
			panic("Missing --topic flag")
		}

		sender, err := kafka.NewKafkaSender(bootstrapUrl, topic)
		if err != nil {
			return nil, err
		}

		loadastic := kafka.NewLoadastic(
			sender,
			kafka.WithInitialWorkers(uint(minWorkers)),
			kafka.WithBeforeSend(func(request kafka.RecordPayload, tickerTimestamp time.Time, id uint64, uuid string) {
				ts, _ := ptypes.TimestampProto(tickerTimestamp)

				sentCh <- performance_common.EventTimestamp{EventId: uuid, At: ts}
			}),
			kafka.WithAfterSend(func(request kafka.RecordPayload, response interface{}, id uint64, uuid string) {
				acceptedCh <- performance_common.EventTimestamp{EventId: uuid, At: ptypes.TimestampNow()}
			}),
		)

		return &KafkaLoadGenerator{loadastic: loadastic, sender: sender}, nil
	}
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomStringFromCharset(length uint, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func JsonKafkaRequestFactory(messageSize uint, messageType string) kafka.RequestFactory {
	return func(tickerTimestamp time.Time, id uint64, uuid string) kafka.RecordPayload {
		return []byte(fmt.Sprintf("{\"type\":\"%s\",\"id\":\"%s\",\"randomStuff\":\"%s\"}", messageType, uuid, randomStringFromCharset(messageSize, charset)))
	}
}

func generatePayloadWithType(t string) kafka.RecordPayload {
	return []byte(fmt.Sprintf("{\"type\":\"%s\"}", t))
}

func paceToStep(pace performance_common.PaceSpec) loadastic_common.Step {
	return loadastic_common.Step{
		Duration: pace.Duration,
		Rps:      uint(pace.Rps),
	}
}

func JsonTypeExtractor(event cloudevents.Event) string {
	var j = make(map[string]interface{})
	_ = event.DataAs(&j)
	var t = j["type"].(string)
	return t
}

func JsonIdExtractor(event cloudevents.Event) string {
	var j = make(map[string]interface{})
	_ = event.DataAs(&j)
	var i = j["id"].(string)
	return i
}
