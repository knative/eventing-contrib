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
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/nats-io/stan.go"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	eventingchannels "knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/kncloudevents"

	"knative.dev/eventing-contrib/natss/pkg/stanutil"

	"github.com/cloudevents/sdk-go/v2/binding"
	natsscloudevents "github.com/cloudevents/sdk-go/v2/protocol/stan"
)

const (
	// maxElements defines a maximum number of outstanding re-connect requests
	maxElements = 10
)

var (
	// retryInterval defines delay in seconds for the next attempt to reconnect to NATSS streaming server
	retryInterval = 1 * time.Second
)

type SubscriptionChannelMapping map[eventingchannels.ChannelReference]map[types.UID]*stan.Subscription

// SubscriptionsSupervisor manages the state of NATS Streaming subscriptions
type SubscriptionsSupervisor struct {
	logger *zap.Logger

	receiver   *eventingchannels.MessageReceiver
	dispatcher *eventingchannels.MessageDispatcherImpl

	subscriptionsMux sync.Mutex
	subscriptions    SubscriptionChannelMapping

	connect   chan struct{}
	natssURL  string
	clusterID string
	clientID  string
	// natConnMux is used to protect natssConn and natssConnInProgress during
	// the transition from not connected to connected states.
	natssConnMux        sync.Mutex
	natssConn           *stan.Conn
	natssConnInProgress bool

	hostToChannelMap atomic.Value
}

type NatssDispatcher interface {
	Start(ctx context.Context) error
	UpdateSubscriptions(ctx context.Context, channel *messagingv1alpha1.Channel, isFinalizer bool) (map[eventingduck.SubscriberSpec]error, error)
	ProcessChannels(ctx context.Context, chanList []messagingv1alpha1.Channel) error
}

type Args struct {
	NatssURL  string
	ClusterID string
	ClientID  string
	Cargs     kncloudevents.ConnectionArgs
	Logger    *zap.Logger
}

// NewDispatcher returns a new NatssDispatcher.
func NewDispatcher(args Args) (NatssDispatcher, error) {
	if args.Logger == nil {
		args.Logger = zap.NewNop()
	}

	d := &SubscriptionsSupervisor{
		logger:        args.Logger,
		dispatcher:    eventingchannels.NewMessageDispatcher(args.Logger),
		subscriptions: make(SubscriptionChannelMapping),
		connect:       make(chan struct{}, maxElements),
		natssURL:      args.NatssURL,
		clusterID:     args.ClusterID,
		clientID:      args.ClientID,
	}

	receiver, err := eventingchannels.NewMessageReceiver(
		messageReceiverFunc(d),
		d.logger,
		eventingchannels.ResolveMessageChannelFromHostHeader(d.getChannelReferenceFromHost))
	if err != nil {
		return nil, err
	}
	d.receiver = receiver
	d.setHostToChannelMap(map[string]eventingchannels.ChannelReference{})
	return d, nil
}

func (s *SubscriptionsSupervisor) signalReconnect() {
	select {
	case s.connect <- struct{}{}:
		// Sent.
	default:
		// The Channel is already full, so a reconnection attempt will occur.
	}
}

func messageReceiverFunc(s *SubscriptionsSupervisor) eventingchannels.UnbufferedMessageReceiverFunc {
	return func(ctx context.Context, channel eventingchannels.ChannelReference, message binding.Message, transformers []binding.Transformer, header http.Header) error {
		s.logger.Info("Received event", zap.String("channel", channel.String()))

		s.natssConnMux.Lock()
		currentNatssConn := s.natssConn
		s.natssConnMux.Unlock()
		if currentNatssConn == nil {
			s.logger.Error("no Connection to NATSS")
			return errors.New("no Connection to NATSS")
		}
		sender, err := natsscloudevents.NewSenderFromConn(*currentNatssConn, getSubject(channel))
		if err != nil {
			s.logger.Error("could not create natss sender", zap.Error(err))
			return errors.Wrap(err, "could not create natss sender")
		}
		if err := sender.Send(ctx, message); err != nil {
			errMsg := "error during send"
			if err.Error() == stan.ErrConnectionClosed.Error() {
				errMsg += " - connection to NATSS has been lost, attempting to reconnect"
				s.signalReconnect()
			}
			s.logger.Error(errMsg, zap.Error(err))
			return errors.Wrap(err, errMsg)
		}
		s.logger.Debug("published", zap.String("channel", channel.String()))
		return nil
	}
}

func (s *SubscriptionsSupervisor) Start(ctx context.Context) error {
	// Starting Connect to establish connection with NATS
	go s.Connect(ctx)
	// Trigger Connect to establish connection with NATS
	s.signalReconnect()
	return s.receiver.Start(ctx)
}

func (s *SubscriptionsSupervisor) connectWithRetry(ctx context.Context) {
	// re-attempting evey 1 second until the connection is established.
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		nConn, err := stanutil.Connect(s.clusterID, s.clientID, s.natssURL, s.logger.Sugar())
		if err == nil {
			// Locking here in order to reduce time in locked state.
			s.natssConnMux.Lock()
			s.natssConn = nConn
			s.natssConnInProgress = false
			s.natssConnMux.Unlock()
			return
		}
		s.logger.Sugar().Errorf("Connect() failed with error: %+v, retrying in %s", err, retryInterval.String())
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return
		}
	}
}

// Connect is called for initial connection as well as after every disconnect
func (s *SubscriptionsSupervisor) Connect(ctx context.Context) {
	for {
		select {
		case <-s.connect:
			s.natssConnMux.Lock()
			currentConnProgress := s.natssConnInProgress
			s.natssConnMux.Unlock()
			if !currentConnProgress {
				// Case for lost connectivity, setting InProgress to true to prevent recursion
				s.natssConnMux.Lock()
				s.natssConnInProgress = true
				s.natssConnMux.Unlock()
				go s.connectWithRetry(ctx)
			}
		case <-ctx.Done():
			return
		}
	}
}

// UpdateSubscriptions creates/deletes the natss subscriptions based on channel.Spec.Subscribable.Subscribers
// Return type:map[eventingduck.SubscriberSpec]error --> Returns a map of subscriberSpec that failed with the value=error encountered.
// Ignore the value in case error != nil
func (s *SubscriptionsSupervisor) UpdateSubscriptions(ctx context.Context, channel *messagingv1alpha1.Channel, isFinalizer bool) (map[eventingduck.SubscriberSpec]error, error) {
	s.subscriptionsMux.Lock()
	defer s.subscriptionsMux.Unlock()

	failedToSubscribe := make(map[eventingduck.SubscriberSpec]error)
	cRef := eventingchannels.ChannelReference{Namespace: channel.Namespace, Name: channel.Name}
	s.logger.Info("Update subscriptions", zap.String("cRef", cRef.String()), zap.String("subscribable", fmt.Sprintf("%v", channel)), zap.Bool("isFinalizer", isFinalizer))
	if channel.Spec.Subscribable == nil || isFinalizer {
		s.logger.Sugar().Infof("Empty subscriptions for channel Ref: %v; unsubscribe all active subscriptions, if any", cRef)
		chMap, ok := s.subscriptions[cRef]
		if !ok {
			// nothing to do
			s.logger.Sugar().Infof("No channel Ref %v found in subscriptions map", cRef)
			return failedToSubscribe, nil
		}
		for sub := range chMap {
			s.logger.Error("unsubscribe", zap.Error(s.unsubscribe(cRef, sub)))
		}
		delete(s.subscriptions, cRef)
		return failedToSubscribe, nil
	}

	subscriptions := channel.Spec.Subscribable.Subscribers
	activeSubs := make(map[types.UID]bool) // it's logically a set

	chMap, ok := s.subscriptions[cRef]
	if !ok {
		chMap = make(map[types.UID]*stan.Subscription)
		s.subscriptions[cRef] = chMap
	}

	var errStrings []string
	for _, sub := range subscriptions {
		// check if the subscription already exist and do nothing in this case
		subRef := newSubscriptionReference(sub)
		if _, ok := chMap[subRef.UID]; ok {
			activeSubs[subRef.UID] = true
			s.logger.Sugar().Infof("Subscription: %v already active for channel: %v", sub, cRef)
			continue
		}
		// subscribe and update failedSubscription if subscribe fails
		natssSub, err := s.subscribe(ctx, cRef, subRef)
		if err != nil {
			errStrings = append(errStrings, err.Error())
			s.logger.Sugar().Errorf("failed to subscribe (subscription:%q) to channel: %v. Error:%s", sub, cRef, err.Error())
			failedToSubscribe[sub] = err
			continue
		}
		chMap[subRef.UID] = natssSub
		activeSubs[subRef.UID] = true
	}
	// Unsubscribe for deleted subscriptions
	for sub := range chMap {
		if ok := activeSubs[sub]; !ok {
			s.logger.Error("unsubscribe", zap.Error(s.unsubscribe(cRef, sub)))
		}
	}
	// delete the channel from s.subscriptions if chMap is empty
	if len(s.subscriptions[cRef]) == 0 {
		delete(s.subscriptions, cRef)
	}
	return failedToSubscribe, nil
}

func (s *SubscriptionsSupervisor) subscribe(ctx context.Context, channel eventingchannels.ChannelReference, subscription subscriptionReference) (*stan.Subscription, error) {
	s.logger.Info("Subscribe to channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	mcb := func(stanMsg *stan.Msg) {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Warn("Panic happened while handling a message",
					zap.String("messages", stanMsg.String()),
					zap.String("sub", string(subscription.UID)),
					zap.Any("panic value", r),
				)
			}
		}()

		message, err := natsscloudevents.NewMessage(stanMsg, natsscloudevents.WithManualAcks())
		if err != nil {
			s.logger.Error("could not create a message", zap.Error(err))
			return
		}
		s.logger.Debug("NATSS message received", zap.String("subject", stanMsg.Subject), zap.Uint64("sequence", stanMsg.Sequence), zap.Time("timestamp", time.Unix(stanMsg.Timestamp, 0)))

		var destination *url.URL
		if !subscription.SubscriberURI.IsEmpty() {
			destination = subscription.SubscriberURI.URL()
			s.logger.Debug("dispatch message", zap.String("destination", destination.String()))
		}
		var reply *url.URL
		if !subscription.ReplyURI.IsEmpty() {
			reply = subscription.ReplyURI.URL()
			s.logger.Debug("dispatch message", zap.String("reply", reply.String()))
		}
		var deadLetter *url.URL
		if subscription.Delivery != nil && subscription.Delivery.DeadLetterSink != nil && !subscription.Delivery.DeadLetterSink.URI.IsEmpty() {
			deadLetter = subscription.Delivery.DeadLetterSink.URI.URL()
			s.logger.Debug("dispatch message", zap.String("deadLetter", deadLetter.String()))
		}
		if !subscription.DeadLetterSinkURI.IsEmpty() {
			deadLetter = subscription.DeadLetterSinkURI.URL()
			s.logger.Debug("dispatch message", zap.String("deadLetter", deadLetter.String()))
		}

		if err := s.dispatcher.DispatchMessage(ctx, message, nil, destination, reply, deadLetter); err != nil {
			s.logger.Error("Failed to dispatch message: ", zap.Error(err))
			return
		}
		if err := stanMsg.Ack(); err != nil {
			s.logger.Error("failed to acknowledge message", zap.Error(err))
		}

		s.logger.Debug("message dispatched", zap.Any("channel", channel))
	}

	ch := getSubject(channel)
	sub := subscription.String()

	s.natssConnMux.Lock()
	currentNatssConn := s.natssConn
	s.natssConnMux.Unlock()

	if currentNatssConn == nil {
		return nil, errors.New("no Connection to NATSS")
	}

	subscriber := &natsscloudevents.RegularSubscriber{}
	natssSub, err := subscriber.Subscribe(*currentNatssConn, ch, mcb, stan.DurableName(sub), stan.SetManualAckMode(), stan.AckWait(1*time.Minute))
	if err != nil {
		s.logger.Error(" Create new NATSS Subscription failed: ", zap.Error(err))
		if err.Error() == stan.ErrConnectionClosed.Error() {
			s.logger.Error("Connection to NATSS has been lost, attempting to reconnect.")
			// Informing SubscriptionsSupervisor to re-establish connection to NATS
			s.signalReconnect()
			return nil, err
		}
		return nil, err
	}

	s.logger.Sugar().Infof("NATSS Subscription created: %+v", natssSub)
	return &natssSub, nil
}

// should be called only while holding subscriptionsMux
func (s *SubscriptionsSupervisor) unsubscribe(channel eventingchannels.ChannelReference, subscription types.UID) error {
	s.logger.Info("Unsubscribe from channel:", zap.Any("channel", channel), zap.Any("subscription", subscription))

	if stanSub, ok := s.subscriptions[channel][subscription]; ok {
		if err := (*stanSub).Unsubscribe(); err != nil {
			s.logger.Error("Unsubscribing NATSS Streaming subscription failed: ", zap.Error(err))
			return err
		}
		delete(s.subscriptions[channel], subscription)
	}
	return nil
}

func getSubject(channel eventingchannels.ChannelReference) string {
	return channel.Name + "." + channel.Namespace
}

func (s *SubscriptionsSupervisor) getHostToChannelMap() map[string]eventingchannels.ChannelReference {
	return s.hostToChannelMap.Load().(map[string]eventingchannels.ChannelReference)
}

func (s *SubscriptionsSupervisor) setHostToChannelMap(hcMap map[string]eventingchannels.ChannelReference) {
	s.hostToChannelMap.Store(hcMap)
}

// NewHostNameToChannelRefMap parses each channel from cList and creates a map[string(Status.Address.HostName)]ChannelReference
func newHostNameToChannelRefMap(cList []messagingv1alpha1.Channel) (map[string]eventingchannels.ChannelReference, error) {
	hostToChanMap := make(map[string]eventingchannels.ChannelReference, len(cList))
	for _, c := range cList {
		url := c.Status.Address.GetURL()
		if cr, present := hostToChanMap[url.Host]; present {
			return nil, fmt.Errorf(
				"duplicate hostName found. Each channel must have a unique host header. HostName:%s, channel:%s.%s, channel:%s.%s",
				url.Host,
				c.Namespace,
				c.Name,
				cr.Namespace,
				cr.Name)
		}
		hostToChanMap[url.Host] = eventingchannels.ChannelReference{Name: c.Name, Namespace: c.Namespace}
	}
	return hostToChanMap, nil
}

// ProcessChannels will be called from the controller that watches natss channels.
// It will update internal hostToChannelMap which is used to resolve the hostHeader of the
// incoming request to the correct ChannelReference in the receiver function.
func (s *SubscriptionsSupervisor) ProcessChannels(ctx context.Context, chanList []messagingv1alpha1.Channel) error {
	s.logger.Debug("ProcessChannels", zap.Any("chanList", chanList))
	hostToChanMap, err := newHostNameToChannelRefMap(chanList)
	if err != nil {
		s.logger.Info("ProcessChannels: Error occurred when creating the new hostToChannel map.", zap.Error(err))
		return err
	}
	s.setHostToChannelMap(hostToChanMap)
	s.logger.Info("hostToChannelMap updated successfully.")
	return nil
}

func (s *SubscriptionsSupervisor) getChannelReferenceFromHost(host string) (eventingchannels.ChannelReference, error) {
	chMap := s.getHostToChannelMap()
	cr, ok := chMap[host]
	if !ok {
		return cr, fmt.Errorf("Invalid HostName:%q. HostName not found in any of the watched natss channels", host)
	}
	return cr, nil
}
