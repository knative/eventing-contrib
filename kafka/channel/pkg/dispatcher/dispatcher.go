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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/sarama"
	cloudevents "github.com/cloudevents/sdk-go/legacy"
	. "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingchannels "knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/channel/swappable"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracing"
)

type KafkaDispatcher struct {
	// TODO: config doesn't have to be atomic as it is read and updated using updateLock.
	config           atomic.Value
	hostToChannelMap atomic.Value
	// hostToChannelMapLock is used to update hostToChannelMap
	hostToChannelMapLock sync.Mutex

	handler    *swappable.Handler
	ceClient   cloudevents.Client
	receiver   *eventingchannels.EventReceiver
	dispatcher *eventingchannels.EventDispatcher

	kafkaAsyncProducer  sarama.AsyncProducer
	kafkaConsumerGroups map[eventingchannels.ChannelReference]map[subscription]sarama.ConsumerGroup
	// consumerUpdateLock must be used to update kafkaConsumers
	consumerUpdateLock   sync.Mutex
	kafkaConsumerFactory kafka.KafkaConsumerGroupFactory

	topicFunc TopicFunc
	logger    *zap.Logger
}

type TopicFunc func(separator, namespace, name string) string

type KafkaDispatcherArgs struct {
	KnCEConnectionArgs *kncloudevents.ConnectionArgs
	Handler            *swappable.Handler
	ClientID           string
	Brokers            []string
	TopicFunc          TopicFunc
	Logger             *zap.Logger
}

type consumerMessageHandler struct {
	sub        subscription
	dispatcher *eventingchannels.EventDispatcher
}

func (c consumerMessageHandler) Handle(ctx context.Context, message *sarama.ConsumerMessage) (bool, error) {
	event := fromKafkaMessage(ctx, message)
	err := c.dispatcher.DispatchEventWithDelivery(ctx, *event, c.sub.SubscriberURI, c.sub.ReplyURI, &c.sub.Delivery)
	// NOTE: only return `true` here if DispatchEventWithDelivery actually delivered the message.
	return err == nil, err
}

var _ kafka.KafkaConsumerHandler = (*consumerMessageHandler)(nil)

type subscription struct {
	Namespace     string
	Name          string
	UID           string
	SubscriberURI string
	ReplyURI      string
	Delivery      eventingchannels.DeliveryOptions
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

	newSubs := make(map[subscription]bool)
	failedToSubscribe := make(map[eventingduck.SubscriberSpec]error)
	for _, cc := range config.ChannelConfigs {
		channelRef := eventingchannels.ChannelReference{
			Name:      cc.Name,
			Namespace: cc.Namespace,
		}
		for _, subSpec := range cc.FanoutConfig.Subscriptions {
			// TODO: use better way to get the provided Name/Namespce for the Subscription
			// we NEED this for better consumer groups
			sub := newSubscription(subSpec, string(subSpec.UID), cc.Namespace)
			if _, ok := d.kafkaConsumerGroups[channelRef][sub]; !ok {
				// only subscribe when not exists in channel-subscriptions map
				// do not need to resubscribe every time channel fanout config is updated
				if err := d.subscribe(channelRef, sub); err != nil {
					failedToSubscribe[subSpec] = err
				}
			}
			newSubs[sub] = true
		}
	}

	// Unsubscribe and close consumer for any deleted subscriptions
	for channelRef, subMap := range d.kafkaConsumerGroups {
		for sub := range subMap {
			if ok := newSubs[sub]; !ok {
				d.unsubscribe(channelRef, sub)
			}
		}
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

	return d.ceClient.StartReceiver(ctx, d.receiver.ServeHTTP)
}

// subscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// subscribe must be called under updateLock.
func (d *KafkaDispatcher) subscribe(channelRef eventingchannels.ChannelReference, sub subscription) error {
	d.logger.Info("Subscribing", zap.Any("channelRef", channelRef), zap.Any("subscription", sub))

	topicName := d.topicFunc(utils.KafkaChannelSeparator, channelRef.Namespace, channelRef.Name)
	groupID := fmt.Sprintf("kafka.%s.%s.%s", sub.Namespace, channelRef.Name, sub.Name)

	handler := &consumerMessageHandler{sub, d.dispatcher}

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
			d.logger.Warn("Error in consumer group", zap.Error(err))
		}
	}()

	consumerGroupMap, ok := d.kafkaConsumerGroups[channelRef]
	if !ok {
		consumerGroupMap = make(map[subscription]sarama.ConsumerGroup)
		d.kafkaConsumerGroups[channelRef] = consumerGroupMap
	}
	consumerGroupMap[sub] = consumerGroup

	return nil
}

// unsubscribe reads kafkaConsumers which gets updated in UpdateConfig in a separate go-routine.
// unsubscribe must be called under updateLock.
func (d *KafkaDispatcher) unsubscribe(channel eventingchannels.ChannelReference, sub subscription) error {
	d.logger.Info("Unsubscribing from channel", zap.Any("channel", channel), zap.Any("subscription", sub))
	if consumer, ok := d.kafkaConsumerGroups[channel][sub]; ok {
		delete(d.kafkaConsumerGroups[channel], sub)
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

func NewDispatcher(args *KafkaDispatcherArgs) (*KafkaDispatcher, error) {
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

	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cloudevents.WithMiddleware(tracing.HTTPSpanIgnoringPaths("/readyz")))
	if err != nil {
		args.Logger.Fatal("failed to create httpTransport", zap.Error(err))
	}

	ceClient, err := kncloudevents.NewDefaultClientGivenHttpTransport(httpTransport, args.KnCEConnectionArgs)
	if err != nil {
		args.Logger.Fatal("failed to create cloudevents client", zap.Error(err))
	}
	dispatcher := &KafkaDispatcher{
		dispatcher:           eventingchannels.NewEventDispatcher(args.Logger),
		kafkaConsumerFactory: kafka.NewConsumerGroupFactory(client),
		kafkaConsumerGroups:  make(map[eventingchannels.ChannelReference]map[subscription]sarama.ConsumerGroup),
		kafkaAsyncProducer:   producer,
		logger:               args.Logger,
		ceClient:             ceClient,
		handler:              args.Handler,
		topicFunc:            args.TopicFunc,
	}
	receiverFunc, err := eventingchannels.NewEventReceiver(
		func(ctx context.Context, channel eventingchannels.ChannelReference, event cloudevents.Event) error {
			dispatcher.kafkaAsyncProducer.Input() <- toKafkaMessage(ctx, channel, event, args.TopicFunc)
			return nil
		},
		args.Logger,
		eventingchannels.ResolveChannelFromHostHeader(eventingchannels.ResolveChannelFromHostFunc(dispatcher.getChannelReferenceFromHost)))
	if err != nil {
		return nil, err
	}

	dispatcher.receiver = receiverFunc
	dispatcher.setConfig(&multichannelfanout.Config{})
	dispatcher.setHostToChannelMap(map[string]eventingchannels.ChannelReference{})
	dispatcher.topicFunc = args.TopicFunc
	return dispatcher, nil
}

func (d *KafkaDispatcher) getChannelReferenceFromHost(host string) (eventingchannels.ChannelReference, error) {
	chMap := d.getHostToChannelMap()
	cr, ok := chMap[host]
	if !ok {
		return cr, fmt.Errorf("invalid Hostname:%s. Hostname not found in ConfigMap for any Channel", host)
	}
	return cr, nil
}

func fromKafkaMessage(ctx context.Context, kafkaMessage *sarama.ConsumerMessage) *cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	for _, header := range kafkaMessage.Headers {
		h := string(header.Key)
		v := string(header.Value)
		logging.FromContext(ctx).Debugf("key: %s, value: %s", h, v)
		switch h {
		case "ce_datacontenttype":
			event.SetDataContentType(v)
		case "ce_specversion":
			event.SetSpecVersion(v)
		case "ce_type":
			event.SetType(v)
		case "ce_source":
			event.SetSource(v)
		case "ce_id":
			event.SetID(v)
		case "ce_time":
			t, _ := time.Parse(time.RFC3339, v)
			event.SetTime(t)
		case "ce_subject":
			event.SetSubject(v)
		case "ce_dataschema":
			event.SetDataSchema(v)
		default:
			// Possible Extensions. Note that we only add headers
			// that start with ce_ to make sure we don't add any
			// additional kafka headers as extensions.
			if strings.HasPrefix(h, "ce_") {
				// if they do not have the ce_ prefix.
				strippedHeader := strings.TrimPrefix(h, "ce_")
				if IsAlphaNumeric(strippedHeader) {
					event.SetExtension(strippedHeader, v)
				}
			}
		}
	}
	event.SetData(kafkaMessage.Value)
	return &event
}

func toKafkaMessage(ctx context.Context, channel eventingchannels.ChannelReference, event cloudevents.Event, topicFunc TopicFunc) *sarama.ProducerMessage {
	data, err := event.DataBytes()
	if err != nil {
		return &sarama.ProducerMessage{} //this should probably be something else indicating an error
	}

	kafkaMessage := sarama.ProducerMessage{
		Topic: topicFunc(utils.KafkaChannelSeparator, channel.Namespace, channel.Name),
		Value: sarama.ByteEncoder(data),
	}
	kafkaMessage = attachKafkaHeaders(kafkaMessage, event)
	return &kafkaMessage
}

func attachKafkaHeaders(message sarama.ProducerMessage, event cloudevents.Event) sarama.ProducerMessage {
	addHeader(&message, "ce_specversion", event.SpecVersion())
	addHeader(&message, "ce_type", event.Type())
	addHeader(&message, "ce_source", event.Source())
	addHeader(&message, "ce_id", event.ID())
	addHeader(&message, "ce_time", event.Time().Format(time.RFC3339))
	if event.DataContentType() != "" {
		addHeader(&message, "ce_datacontenttype", event.DataContentType())
	}
	if event.Subject() != "" {
		addHeader(&message, "ce_subject", event.Subject())
	}
	if event.DataSchema() != "" {
		addHeader(&message, "ce_dataschema", event.DataSchema())
	}
	// Only setting string extensions.
	for k, v := range event.Extensions() {
		if vs, ok := v.(string); ok {
			addHeader(&message, "ce_"+k, vs)
		}
	}
	return message
}

func addHeader(kafkaMessage *sarama.ProducerMessage, key, value string) {
	kafkaMessage.Headers = append(kafkaMessage.Headers, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func newSubscription(spec eventingduck.SubscriberSpec, name string, namespace string) subscription {
	sub := subscription{
		Name:          name,
		Namespace:     namespace,
		UID:           string(spec.UID),
		SubscriberURI: spec.SubscriberURI.String(),
		ReplyURI:      spec.ReplyURI.String(),
	}
	if spec.Delivery != nil {
		sub.Delivery = eventingchannels.DeliveryOptions{
			DeadLetterSink: spec.DeadLetterSinkURI.String(),
		}
	}
	return sub
}
