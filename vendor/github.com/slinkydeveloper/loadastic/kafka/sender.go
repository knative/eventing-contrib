package kafka

import (
	"github.com/Shopify/sarama"
	"math/rand"
	"strings"
	"time"
)

type RecordPayload []byte

type KafkaSender struct {
	topic string

	producer sarama.AsyncProducer
}

func NewKafkaSender(bootstrapUrl string, topicName string) (Sender, error) {
	config := sarama.NewConfig()

	config.Net.MaxOpenRequests = 100
	config.Net.KeepAlive = 10 * time.Second
	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = false
	config.Producer.Flush.MaxMessages = 5
	config.Version = sarama.V2_0_0_0

	producer, err := sarama.NewAsyncProducer(strings.Split(bootstrapUrl, ","), config)
	if err != nil {
		return nil, err
	}

	return &KafkaSender{
		topic:    topicName,
		producer: producer,
	}, nil
}

func (ks KafkaSender) Send(payload RecordPayload) (interface{}, error) {
	message := sarama.ProducerMessage{
		Topic: ks.topic,
		Value: sarama.ByteEncoder(payload),
	}

	ks.producer.Input() <- &message

	return nil, nil
}

func (ks KafkaSender) Close() {
	_ = ks.producer.Close()
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

func RandomRequestFactory(messageSize uint) RequestFactory {
	rand.Seed(time.Now().UnixNano())

	return func(tickerTimestamp time.Time, id uint64, uuid string) RecordPayload {
		return []byte(randomStringFromCharset(messageSize, charset))
	}
}
