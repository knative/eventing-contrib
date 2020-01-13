package testing

import (
	"context"
	"testing"

	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	logtesting "knative.dev/pkg/logging/testing"

	"knative.dev/eventing-contrib/natss/pkg/dispatcher"
)

type SubscriptionsSupervisorTesting struct {
	logger *zap.Logger
}

var _ dispatcher.NatssDispatcher = (*SubscriptionsSupervisorTesting)(nil)

func NewTestingDispatcher(t *testing.T) dispatcher.NatssDispatcher {
	return &SubscriptionsSupervisorTesting{logger: logtesting.TestLogger(t).Desugar()}
}

func (s *SubscriptionsSupervisorTesting) Start(stopCh <-chan struct{}) error {
	s.logger.Info("start")
	return nil
}

func (s *SubscriptionsSupervisorTesting) UpdateSubscriptions(channel *messagingv1alpha1.Channel, isFinalizer bool) (map[eventingduck.SubscriberSpec]error, error) {
	s.logger.Info("updating subscriptions")
	return nil, nil
}

func (s *SubscriptionsSupervisorTesting) UpdateHostToChannelMap(ctx context.Context, chanList []messagingv1alpha1.Channel) error {
	s.logger.Info("updating hosttochannel map")
	return nil
}
