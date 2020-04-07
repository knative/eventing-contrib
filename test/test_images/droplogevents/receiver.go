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

package droplogevents

import (
	"context"
	"errors"
	"log"
	nethttp "net/http"
	"sync"

	"github.com/cloudevents/sdk-go/v2/binding"

	"go.uber.org/zap"

	"knative.dev/pkg/signals"

	eventingchannels "knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/tracing"
)

var SkipMessage = errors.New("skip message")

// Skipper represents the logic to apply to accept/reject events.
type Skipper interface {
	// Skip returns true if the message must be rejected
	Skip(counter uint64) bool
}

func StartReceiver(skipper Skipper) {
	ctx := signals.NewContext()

	logger, _ := zap.NewDevelopment()
	if err := tracing.SetupStaticPublishing(logger.Sugar(), "", tracing.AlwaysSample); err != nil {
		log.Fatalf("Unable to setup trace publishing: %v", err)
	}

	receiver, err := eventingchannels.NewMessageReceiver(NewHandler(skipper), logger)
	if err != nil {
		log.Fatal("Unable to create message receiver", err)
	}
	log.Print("Start receiver")
	log.Fatal(receiver.Start(ctx))
}

type CounterHandler struct {
	counter uint64
	skipper Skipper
	sync.Mutex
}

func NewHandler(skipper Skipper) eventingchannels.UnbufferedMessageReceiverFunc {
	h := &CounterHandler{
		skipper: skipper,
	}
	return func(ctx context.Context, reference eventingchannels.ChannelReference, message binding.Message, factories []binding.TransformerFactory, header nethttp.Header) error {
		if h.skip() {
			log.Print(SkipMessage)
			_ = message.Finish(SkipMessage)
			return SkipMessage
		}
		event, err := binding.ToEvent(ctx, message, factories...)
		if err != nil {
			log.Fatal("Unable to get event from message", err)
		}
		log.Print(event.String())
		return message.Finish(nil)
	}
}

func (h *CounterHandler) skip() bool {
	h.Lock()
	defer h.Unlock()

	h.counter++
	return h.skipper.Skip(h.counter)
}
