/*
Copyright 2019 The Knative Authors

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

package kafka

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

//------ Mocks

var mockMessage = sarama.ConsumerMessage{
	Headers: []*sarama.RecordHeader{
		{
			Key:   []byte("k1"),
			Value: []byte("v1"),
		},
	},
	Value: []byte("data"),
}

type mockConsumerGroupSession struct {
	marked bool
}

func (m *mockConsumerGroupSession) Claims() map[string][]int32 {
	return nil
}

func (m *mockConsumerGroupSession) MemberID() string {
	return ""
}

func (m *mockConsumerGroupSession) GenerationID() int32 {
	return 0
}

func (m *mockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
}

func (m *mockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
}

func (m *mockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	m.marked = true
}

func (m *mockConsumerGroupSession) Context() context.Context {
	return nil
}

var _ sarama.ConsumerGroupSession = (*mockConsumerGroupSession)(nil)

type mockConsumerGroupClaim struct {
	msg *sarama.ConsumerMessage
}

func (m mockConsumerGroupClaim) Topic() string {
	return ""
}

func (m mockConsumerGroupClaim) Partition() int32 {
	return 0
}

func (m mockConsumerGroupClaim) InitialOffset() int64 {
	return 0
}

func (m mockConsumerGroupClaim) HighWaterMarkOffset() int64 {
	return 0
}

func (m mockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	c := make(chan *sarama.ConsumerMessage, 1)
	c <- m.msg
	defer close(c)
	return c
}

type mockMessageHandler struct {
	shouldErr  bool
	shouldMark bool
}

func (m mockMessageHandler) Handle(ctx context.Context, message *sarama.ConsumerMessage) (bool, error) {
	if m.shouldErr {
		return m.shouldMark, errors.New("bla")
	} else {
		return m.shouldMark, nil
	}
}

//------ Tests

func Test(t *testing.T) {
	tests := []mockMessageHandler{
		{
			shouldErr:  false,
			shouldMark: true,
		},
		{
			shouldErr:  true,
			shouldMark: true,
		},
		{
			shouldErr:  true,
			shouldMark: false,
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("shouldErr: %v, shouldMark: %v", test.shouldErr, test.shouldMark), func(t *testing.T) {
			cgh := NewConsumerHandler(zap.NewNop(), test)

			session := mockConsumerGroupSession{}
			claim := mockConsumerGroupClaim{msg: &mockMessage}

			_ = cgh.Setup(&session)
			_ = cgh.ConsumeClaim(&session, claim)

			if test.shouldErr {
				e := <-cgh.errors
				if e.Error() != "bla" {
					t.Errorf("Wrong error received %v", e)
				}
			}

			if test.shouldMark {
				if !session.marked {
					t.Errorf("Session was not marked")
				}
			}

			_ = cgh.Cleanup(&session)

		})
	}
}
