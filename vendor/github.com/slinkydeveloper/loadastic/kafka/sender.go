package kafka

import (
	"github.com/Shopify/sarama"
	"math/rand"
	"strings"
	"time"
)

type RecordPayload []byte

type KafkaWorker struct {
	producer sarama.AsyncProducer
}

type KafkaSender struct {
	topic  string
	client sarama.Client
}

func NewKafkaSender(bootstrapUrl string, topicName string) (Sender, error) {
	config := sarama.NewConfig()

	config.Producer.Partitioner = sarama.NewRandomPartitioner
	config.Producer.Return.Successes = false
	config.Version = sarama.V2_0_0_0

	client, err := sarama.NewClient(strings.Split(bootstrapUrl, ","), config)
	if err != nil {
		return nil, err
	}

	return &KafkaSender{
		topic:  topicName,
		client: client,
	}, nil
}

func NewKafkaSenderFromSaramaClient(client sarama.Client, topicName string) (Sender, error) {
	return &KafkaSender{
		topic:  topicName,
		client: client,
	}, nil
}

func (ks *KafkaSender) InitializeWorker() *KafkaWorker {
	producer, _ := sarama.NewAsyncProducerFromClient(ks.client)
	return &KafkaWorker{producer: producer}
}

func (ks *KafkaSender) Send(worker *KafkaWorker, payload RecordPayload) (interface{}, error) {
	message := sarama.ProducerMessage{
		Topic: ks.topic,
		Value: sarama.ByteEncoder(payload),
	}

	worker.producer.Input() <- &message

	return nil, nil
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
