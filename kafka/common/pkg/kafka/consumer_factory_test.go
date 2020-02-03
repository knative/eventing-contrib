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
	"testing"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

//------ Mocks

type mockConsumerGroup struct {
	mockInputMessageCh             chan *sarama.ConsumerMessage
	mustGenerateConsumerGroupError bool
	mustGenerateHandlerError       bool
	consumeMustReturnError         bool
}

func (m mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	if m.mustGenerateHandlerError {
		go func() {
			h := handler.(*saramaConsumerHandler)
			h.errors <- errors.New("cgh")
			_ = h.Cleanup(nil)
		}()
	}
	if m.consumeMustReturnError {
		return errors.New("boom!")
	}
	return nil
}

func (m mockConsumerGroup) Errors() <-chan error {
	ch := make(chan error)
	go func() {
		if m.mustGenerateConsumerGroupError {
			ch <- errors.New("cg")
		}
		close(ch)
	}()
	return ch
}

func (m mockConsumerGroup) Close() error {
	return nil
}

func mockedNewConsumerGroupFromClient(mockInputMessageCh chan *sarama.ConsumerMessage, mustGenerateConsumerGroupError bool, mustGenerateHandlerError bool, consumeMustReturnError bool, mustFail bool) func(groupID string, client sarama.Client) (group sarama.ConsumerGroup, e error) {
	if !mustFail {
		return func(groupID string, client sarama.Client) (group sarama.ConsumerGroup, e error) {
			return mockConsumerGroup{
				mockInputMessageCh:             mockInputMessageCh,
				mustGenerateConsumerGroupError: mustGenerateConsumerGroupError,
				mustGenerateHandlerError:       mustGenerateHandlerError,
				consumeMustReturnError:         consumeMustReturnError,
			}, nil
		}
	} else {
		return func(groupID string, client sarama.Client) (group sarama.ConsumerGroup, e error) {
			return nil, errors.New("failed")
		}
	}
}

//------ Tests

func TestErrorPropagationCustomConsumerGroup(t *testing.T) {
	// Mock newConsumerGroupFromClient to return our custom stuff
	newConsumerGroupFromClient = mockedNewConsumerGroupFromClient(nil, true, true, false, false)

	factory := NewConsumerGroupFactory(nil)
	consumerGroup, err := factory.StartConsumerGroup("bla", []string{}, zap.NewNop(), nil)

	if err != nil {
		t.Errorf("Should not throw error %v", err)
	}

	errorsSlice := make([]error, 0)

	for e := range consumerGroup.Errors() {
		errorsSlice = append(errorsSlice, e)
	}

	if len(errorsSlice) != 2 {
		t.Errorf("len(errorsSlice) != 2")
	}

	assertContainsError(t, errorsSlice, "cgh")
	assertContainsError(t, errorsSlice, "cg")
}

func assertContainsError(t *testing.T, collection []error, errorStr string) {
	for _, el := range collection {
		if el.Error() == errorStr {
			return
		}
	}
	t.Errorf("Cannot find error %v in error collection %v", errorStr, collection)
}

func TestErrorWhileCreatingNewConsumerGroup(t *testing.T) {
	// Mock newConsumerGroupFromClient to return our custom stuff
	newConsumerGroupFromClient = mockedNewConsumerGroupFromClient(nil, true, true, false, true)

	factory := NewConsumerGroupFactory(nil)
	_, err := factory.StartConsumerGroup("bla", []string{}, zap.L(), nil)

	if err == nil || err.Error() != "failed" {
		t.Errorf("Should contain an error with message failed. Got %v", err)
	}
}

func TestErrorWhileNewConsumerGroup(t *testing.T) {
	// Mock newConsumerGroupFromClient to return our custom stuff
	newConsumerGroupFromClient = mockedNewConsumerGroupFromClient(nil, false, false, true, false)

	factory := NewConsumerGroupFactory(nil)
	cg, _ := factory.StartConsumerGroup("bla", []string{}, zap.L(), nil)

	err := <-cg.Errors()

	if err == nil || err.Error() != "boom!" {
		t.Errorf("Should contain an error with message boom!. Got %v", err)
	}
}
