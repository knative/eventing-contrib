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
	"sync"
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
	generateErrorOnce              sync.Once
}

func (m *mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	if m.mustGenerateHandlerError {
		go func() {
			m.generateErrorOnce.Do(func() {
				h := handler.(*saramaConsumerHandler)
				h.errors <- errors.New("cgh")
				_ = h.Cleanup(nil)
			})
		}()
	}
	if m.consumeMustReturnError {
		return errors.New("boom!")
	}
	return nil
}

func (m *mockConsumerGroup) Errors() <-chan error {
	ch := make(chan error)
	go func() {
		if m.mustGenerateConsumerGroupError {
			ch <- errors.New("cg")
		}
		close(ch)
	}()
	return ch
}

func (m *mockConsumerGroup) Close() error {
	return nil
}

func mockedNewConsumerGroupFromClient(mockInputMessageCh chan *sarama.ConsumerMessage, mustGenerateConsumerGroupError bool, mustGenerateHandlerError bool, consumeMustReturnError bool, mustFail bool) func(addrs []string, groupID string, config *sarama.Config) (sarama.ConsumerGroup, error) {
	if !mustFail {
		return func(addrs []string, groupID string, config *sarama.Config) (sarama.ConsumerGroup, error) {
			return &mockConsumerGroup{
				mockInputMessageCh:             mockInputMessageCh,
				mustGenerateConsumerGroupError: mustGenerateConsumerGroupError,
				mustGenerateHandlerError:       mustGenerateHandlerError,
				consumeMustReturnError:         consumeMustReturnError,
				generateErrorOnce:              sync.Once{},
			}, nil
		}
	} else {
		return func(addrs []string, groupID string, config *sarama.Config) (sarama.ConsumerGroup, error) {
			return nil, errors.New("failed")
		}
	}
}

//------ Tests

func TestErrorPropagationCustomConsumerGroup(t *testing.T) {

	factory := kafkaConsumerGroupFactoryImpl{
		config:               sarama.NewConfig(),
		addrs:                []string{"b1", "b2"},
		newConsumerGroupFunc: mockedNewConsumerGroupFromClient(nil, true, true, false, false),
	}

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
	factory := kafkaConsumerGroupFactoryImpl{
		config:               sarama.NewConfig(),
		addrs:                []string{"b1", "b2"},
		newConsumerGroupFunc: mockedNewConsumerGroupFromClient(nil, true, true, false, true),
	}
	_, err := factory.StartConsumerGroup("bla", []string{}, zap.L(), nil)

	if err == nil || err.Error() != "failed" {
		t.Errorf("Should contain an error with message failed. Got %v", err)
	}
}

func TestErrorWhileNewConsumerGroup(t *testing.T) {
	factory := kafkaConsumerGroupFactoryImpl{
		config:               sarama.NewConfig(),
		addrs:                []string{"b1", "b2"},
		newConsumerGroupFunc: mockedNewConsumerGroupFromClient(nil, false, false, true, false),
	}
	cg, _ := factory.StartConsumerGroup("bla", []string{}, zap.L(), nil)

	err := <-cg.Errors()

	if err == nil || err.Error() != "boom!" {
		t.Errorf("Should contain an error with message boom!. Got %v", err)
	}
}

type saramaClientMock struct{}

func (s saramaClientMock) Config() *sarama.Config {
	return sarama.NewConfig()
}

func (s saramaClientMock) Controller() (*sarama.Broker, error) {
	return sarama.NewBroker(""), nil
}

func (s saramaClientMock) Brokers() []*sarama.Broker {
	return []*sarama.Broker{sarama.NewBroker("")}
}

func (s saramaClientMock) Topics() ([]string, error) {
	panic("implement me")
}

func (s saramaClientMock) Partitions(_ string) ([]int32, error) {
	panic("implement me")
}

func (s saramaClientMock) WritablePartitions(_ string) ([]int32, error) {
	panic("implement me")
}

func (s saramaClientMock) Leader(_ string, _ int32) (*sarama.Broker, error) {
	panic("implement me")
}

func (s saramaClientMock) Replicas(_ string, _ int32) ([]int32, error) {
	panic("implement me")
}

func (s saramaClientMock) InSyncReplicas(_ string, _ int32) ([]int32, error) {
	panic("implement me")
}

func (s saramaClientMock) OfflineReplicas(_ string, _ int32) ([]int32, error) {
	panic("implement me")
}

func (s saramaClientMock) RefreshMetadata(_ ...string) error {
	panic("implement me")
}

func (s saramaClientMock) GetOffset(_ string, _ int32, _ int64) (int64, error) {
	panic("implement me")
}

func (s saramaClientMock) Coordinator(_ string) (*sarama.Broker, error) {
	panic("implement me")
}

func (s saramaClientMock) RefreshCoordinator(_ string) error {
	panic("implement me")
}

func (s saramaClientMock) InitProducerID() (*sarama.InitProducerIDResponse, error) {
	panic("implement me")
}

func (s saramaClientMock) Close() error {
	panic("implement me")
}

func (s saramaClientMock) Closed() bool {
	panic("implement me")
}

var _ sarama.Client = saramaClientMock{}
