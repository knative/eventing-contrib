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
package dispatcher

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/v2/binding"
	protocolkafka "github.com/cloudevents/sdk-go/v2/protocol/kafka_sarama"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingchannels "knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"
)

type KafkaDispatcher struct {
	// TODO: config doesn't have to be atomic as it is read and updated using updateLock.
	config           atomic.Value
	hostToChannelMap atomic.Value
	// hostToChannelMapLock is used to update hostToChannelMap
	hostToChannelMapLock sync.Mutex

	messageSender *kncloudevents.HttpMessageSender
	receiver      *eventingchannels.MessageReceiver
	dispatcher    *eventingchannels.MessageDispatcherImpl

	kafkaAsyncProducer   sarama.AsyncProducer
	channelSubscriptions map[eventingchannels.ChannelReference][]types.UID
	subsConsumerGroups   map[types.UID]sarama.ConsumerGroup
	subscriptions        map[types.UID]subscription
	// consumerUpdateLock must be used to update kafkaConsumers
	consumerUpdateLock   sync.Mutex
	kafkaConsumerFactory kafka.KafkaConsumerGroupFactory

	topicFunc TopicFunc
	logger    *zap.Logger
}

func NewDispatcher(ctx context.Context, args *KafkaDispatcherArgs) (*KafkaDispatcher, error) {
	conf := sarama.NewConfig()
	conf.Version = sarama.V2_0_0_0
	conf.ClientID = args.ClientID
	conf.Consumer.Return.Errors = true // Returns the errors in ConsumerGroup#Errors() https://godoc.org/github.com/Shopify/sarama#ConsumerGroup
	client, err := sarama.NewClient(args.Brokers, conf)

	if err != nil {
		return nil, fmt.Errorf("unable to create kafka client: %v", err)
	}

	producer, err := sarama.NewAsyncProducerFromClient(client)
	if err != nil {
		return nil, fmt.Errorf("unable to create kafka producer: %v", err)
	}

	messageSender, err := kncloudevents.NewHttpMessageSender(args.KnCEConnectionArgs, "")
	if err != nil {
		args.Logger.Fatal("failed to create message sender", zap.Error(err))
	}

	dispatcher := &KafkaDispatcher{
		dispatcher:           eventingchannels.NewMessageDispatcher(args.Logger),
		kafkaConsumerFactory: kafka.NewConsumerGroupFactory(client),
		channelSubscriptions: make(map[eventingchannels.ChannelReference][]types.UID),
		subsConsumerGroups:   make(map[types.UID]sarama.ConsumerGroup),
		subscriptions:        make(map[types.UID]subscription),
		kafkaAsyncProducer:   producer,
		logger:               args.Logger,
		messageSender:        messageSender,
		topicFunc:            args.TopicFunc,
	}
	receiverFunc, err := eventingchannels.NewMessageReceiver(
		func(ctx context.Context, channel eventingchannels.ChannelReference, message binding.Message, transformers []binding.Transformer, _ nethttp.Header) error {
			kafkaProducerMessage := sarama.ProducerMessage{
				Topic: dispatcher.topicFunc(utils.KafkaChannelSeparator, channel.Namespace, channel.Name),
			}

			dispatcher.logger.Debug("Received a new message from MessageReceiver, dispatching to Kafka", zap.Any("channel", channel))
			err := protocolkafka.WriteProducerMessage(ctx, message, &kafkaProducerMessage, transformers...)
			if err != nil {
				return err
			}

			dispatcher.kafkaAsyncProducer.Input() <- &kafkaProducerMessage
			return nil
		},
		args.Logger,
		eventingchannels.ResolveMessageChannelFromHostHeader(dispatcher.getChannelReferenceFromHost))
	if err != nil {
		return nil, err
	}

	dispatcher.receiver = receiverFunc
	dispatcher.setConfig(&multichannelfanout.Config{})
	dispatcher.setHostToChannelMap(map[string]eventingchannels.ChannelReference{})
	return dispatcher, nil
}

type TopicFunc func(separator, namespace, name string) string

type KafkaDispatcherArgs struct {
	KnCEConnectionArgs *kncloudevents.ConnectionArgs
	ClientID           string
	Brokers            []string
	TopicFunc          TopicFunc
	Logger             *zap.Logger
}

type consumerMessageHandler struct {
	logger     *zap.Logger
	sub        subscription
	dispatcher *eventingchannels.MessageDispatcherImpl
}

func (c consumerMessageHandler) Handle(ctx context.Context, consumerMessage *sarama.ConsumerMessage) (bool, error) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Warn("Panic happened while handling a message",
				zap.String("topic", consumerMessage.Topic),
				zap.String("sub", string(c.sub.UID)),
				zap.Any("panic value", r),
			)
		}
	}()
	message := protocolkafka.NewMessageFromConsumerMessage(consumerMessage)
	if message.ReadEncoding() == binding.EncodingUnknown {
		return false, errors.New("received a message with unknown encoding")
	}
	var destination *url.URL
	if !c.sub.SubscriberURI.IsEmpty() {
		destination = c.sub.SubscriberURI.URL()
	}
	var reply *url.URL
	if !c.sub.ReplyURI.IsEmpty() {
		reply = c.sub.ReplyURI.URL()
	}
	var deadLetter *url.URL
	if c.sub.Delivery != nil && c.sub.Delivery.DeadLetterSink != nil && !c.sub.Delivery.DeadLetterSink.URI.IsEmpty() {
		deadLetter = c.sub.Delivery.DeadLetterSink.URI.URL()
	}
	c.logger.Debug("Going to dispatch the message",
		zap.String("topic", consumerMessage.Topic),
		zap.String("sub", string(c.sub.UID)),
	)
	err := c.dispatcher.DispatchMessage(context.Background(), message, nil, destination, reply, deadLetter)
	// NOTE: only return `true` here if DispatchMessage actually delivered the message.
	return err == nil, err
}

var _ kafka.KafkaConsumerHandler = (*consumerMessageHandler)(nil)

type subscription struct {
	eventingduck.SubscriberSpec
	Namespace string
	Name      string
}

// configDiff diffs the new config with the existing config. If there are no differences, then the
// empty string is returned. If there are differences, then a non-empty string is returned
// describing the differences.
func (d *KafkaDispatcher) configDiff(updated *multichannelfanout.Config) string {
	return cmp.Diff(d.getConfig(), updated)
}

// UpdateKafkaConsumers will be called by new CRD based kafka channel dispatcher controller.
func (d *KafkaDispatcher) UpdateKafkaConsumers(config *multichannelfanout.Config) (map[eventingduck.SubscriberSpec]error, error) {
	if config == nil {
		return nil, fmt.Errorf("nil config")
	}

	d.consumerUpdateLock.Lock()
	defer d.consumerUpdateLock.Unlock()

	var newSubs []types.UID
	failedToSubscribe := make(map[eventingduck.SubscriberSpec]error)
	for _, cc := range config.ChannelConfigs {
		channelRef := eventingchannels.ChannelReference{
			Name:      cc.Name,
			Namespace: cc.Namespace,
		}
		for _, subSpec := range cc.FanoutConfig.Subscriptions {
			sub := newSubscription(subSpec, string(subSpec.UID), cc.Namespace)
			newSubs = append(newSubs, sub.UID)

			// Check if sub already exists
			exists := false
			for _, s := range d.channelSubscriptions[channelRef] {
				if s == sub.UID {
					exists = true
				}
			}

			if !exists {
				// only subscribe when not exists in channel-subscriptions map
				// do not need to resubscribe every time channel fanout config is updated
				if err := d.subscribe(channelRef, sub); err != nil {
					failedToSubscribe[subSpec] = err
				}
			}
		}
	}

	d.logger.Debug("Number of new subs", zap.Any("subs", len(newSubs)))
	d.logger.Debug("Number of subs failed to subscribe", zap.Any("subs", len(failedToSubscribe)))

	// Unsubscribe and close consumer for any deleted subscriptions
	for channelRef, subs := range d.channelSubscriptions {
		for _, oldSub := range subs {
			removedSub := true
			for _, s := range newSubs {
				if s == oldSub {
					removedSub = false
				}
			}

			if removedSub {
				if err := d.unsubscribe(channelRef, d.subscriptions[oldSub]); err != nil {
					return nil, err
				}
			}
		}
		d.channelSubscriptions[channelRef] = newSubs
	}
	return failedToSubscribe, nil
}

// UpdateHostToChannelMap will be called by new CRD based kafka channel dispatcher controller.
func (d *KafkaDispatcher) UpdateHostToChannelMap(config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	d.hostToChannelMapLock.Lock()
	defer d.hostToChannelMapLock.Unlock()

	hcMap, err := createHostToChannelMap(config)
	if err != nil {
		return err
	}

	d.setHostToChannelMap(hcMap)
	return nil
}

func createHostToChannelMap(config *multichannelfanout.Config) (map[string]eventingchannels.ChannelReference, error) {
	hcMap := make(map[string]eventingchannels.ChannelReference, len(config.ChannelConfigs))
	for _, cConfig := range config.ChannelConfigs {
		if cr, ok := hcMap[cConfig.HostName]; ok {
			return nil, fmt.Errorf(
				"duplicate hostName found. Each channel must have a unique host header. HostName:%s, channel:%s.%s, channel:%s.%s",
				cConfig.HostName,
				cConfig.Namespace,
				cConfig.Name,
				cr.Namespace,
				cr.Name)
		}
		hcMap[cConfig.HostName] = eventingchannels.ChannelReference{Name: cConfig.Name, Namespace: cConfig.Namespace}
	}
	return hcMap, nil
}

// Start starts the kafka dispatcher's message processing.
func (d *KafkaDispatcher) Start(ctx context.Context) error {
	if d.receiver == nil {
		return fmt.Errorf("message receiver is not set")
	}

	if d.kafkaAsyncProducer == nil {
		return fmt.Errorf("kafkaAsyncProducer is not set")
	}

	go func() {
		for {
			select {
			case e := <-d.kafkaAsyncProducer.Errors():
				d.logger.Warn("Got", zap.Error(e))
			case s := <-d.kafkaAsyncProducer.Successes():
				d.logger.Info("Sent", zap.Any("success", s))
			case <-ctx.Done():
				return
			}
		}
	}()

	return d.receiver.Start(ctx)
}

// subscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// subscribe must be called under updateLock.
func (d *KafkaDispatcher) subscribe(channelRef eventingchannels.ChannelReference, sub subscription) error {
	d.logger.Info("Subscribing", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))

	topicName := d.topicFunc(utils.KafkaChannelSeparator, channelRef.Namespace, channelRef.Name)
	groupID := fmt.Sprintf("kafka.%s.%s.%s", sub.Namespace, channelRef.Name, sub.Name)

	handler := &consumerMessageHandler{d.logger, sub, d.dispatcher}

	consumerGroup, err := d.kafkaConsumerFactory.StartConsumerGroup(groupID, []string{topicName}, d.logger, handler)

	if err != nil {
		// we can not create a consumer - logging that, with reason
		d.logger.Info("Could not create proper consumer", zap.Error(err))
		return err
	}

	// sarama reports error in consumerGroup.Error() channel
	// this goroutine logs errors incoming
	go func() {
		for err = range consumerGroup.Errors() {
			panic(err)
		}
	}()

	d.channelSubscriptions[channelRef] = append(d.channelSubscriptions[channelRef], sub.UID)
	d.subscriptions[sub.UID] = sub
	d.subsConsumerGroups[sub.UID] = consumerGroup

	return nil
}

// unsubscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// unsubscribe must be called under updateLock.
func (d *KafkaDispatcher) unsubscribe(channel eventingchannels.ChannelReference, sub subscription) error {
	d.logger.Info("Unsubscribing from channel", zap.Any("channel", channel), zap.Any("subscription", sub))
	delete(d.subscriptions, sub.UID)
	if subsSlice, ok := d.channelSubscriptions[channel]; ok {
		var newSlice []types.UID
		for _, oldSub := range subsSlice {
			if oldSub != sub.UID {
				newSlice = append(newSlice, oldSub)
			}
		}
		d.channelSubscriptions[channel] = newSlice
	}
	if consumer, ok := d.subsConsumerGroups[sub.UID]; ok {
		delete(d.subsConsumerGroups, sub.UID)
		return consumer.Close()
	}
	return nil
}
func (d *KafkaDispatcher) getConfig() *multichannelfanout.Config {
	return d.config.Load().(*multichannelfanout.Config)
}

func (d *KafkaDispatcher) setConfig(config *multichannelfanout.Config) {
	d.config.Store(config)
}

func (d *KafkaDispatcher) getHostToChannelMap() map[string]eventingchannels.ChannelReference {
	return d.hostToChannelMap.Load().(map[string]eventingchannels.ChannelReference)
}

func (d *KafkaDispatcher) setHostToChannelMap(hcMap map[string]eventingchannels.ChannelReference) {
	d.hostToChannelMap.Store(hcMap)
}

func (d *KafkaDispatcher) getChannelReferenceFromHost(host string) (eventingchannels.ChannelReference, error) {
	chMap := d.getHostToChannelMap()
	cr, ok := chMap[host]
	if !ok {
		return cr, eventingchannels.UnknownHostError(host)
	}
	return cr, nil
}

func newSubscription(spec eventingduck.SubscriberSpec, name string, namespace string) subscription {
	return subscription{
		SubscriberSpec: spec,
		Name:           name,
		Namespace:      namespace,
	}
}
