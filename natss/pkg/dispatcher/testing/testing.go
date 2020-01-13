/*
Copyright 2020 The Knative Authors

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

func NewDispatcher(t *testing.T) dispatcher.NatssDispatcher {
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
