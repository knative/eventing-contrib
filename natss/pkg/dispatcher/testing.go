package dispatcher

import (
	"context"
	"testing"

	stan "github.com/nats-io/go-nats-streaming"
	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	eventingchannels "knative.dev/eventing/pkg/channel"
	logtesting "knative.dev/pkg/logging/testing"
)


type SubscriptionsSupervisorTesting struct {
	subscriptions SubscriptionChannelMapping
	logger *zap.Logger
}

//var _ SubscriptionsSupervisorTesting = (*SubscriptionsSupervisor)(nil)

func NewTestingDispatcher(t *testing.T, subscriptions SubscriptionChannelMapping) (*SubscriptionsSupervisor, error) {
	natssDispatcher := &SubscriptionsSupervisorTesting{subscriptions: subscriptions, logger: logtesting.TestLogger(t).Desugar()}
	//natssDispatcher.subscriptions = subscriptions
	return natssDispatcher, nil
}

func (s *SubscriptionsSupervisorTesting) Start(stopCh <-chan struct{}) error {
	return nil
}

func (s *SubscriptionsSupervisorTesting) connectWithRetry(stopCh <-chan struct{}) {}
func (s *SubscriptionsSupervisorTesting) Connect(stopCh <-chan struct{})          {}

func (s *SubscriptionsSupervisorTesting) UpdateSubscriptions(channel *messagingv1alpha1.Channel, isFinalizer bool) (map[eventingduck.SubscriberSpec]error, error) {
	return nil, nil
}

func (s *SubscriptionsSupervisorTesting) subscribe(channel eventingchannels.ChannelReference, subscription subscriptionReference) (*stan.Subscription, error) {
	return nil, nil
}

func (s *SubscriptionsSupervisorTesting) unsubscribe(channel eventingchannels.ChannelReference, subscription subscriptionReference) error {
	return nil
}

func (s *SubscriptionsSupervisorTesting) getHostToChannelMap() map[string]eventingchannels.ChannelReference {
	return nil
}

func (s *SubscriptionsSupervisorTesting) setHostToChannelMap(hcMap map[string]eventingchannels.ChannelReference) {
}

func (s *SubscriptionsSupervisorTesting) UpdateHostToChannelMap(ctx context.Context, chanList []messagingv1alpha1.Channel) error {
	return nil
}

func (s *SubscriptionsSupervisorTesting) getChannelReferenceFromHost(host string) (eventingchannels.ChannelReference, error) {
	return eventingchannels.ChannelReference{}, nil
}
