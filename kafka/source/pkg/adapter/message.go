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
package kafka

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	nethttp "net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/Shopify/sarama"
	protocolkafka "github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/linkedin/goavro/v2"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
)

type AvroDecoder struct {
	schemaURL string
	codecMap  map[int]*goavro.Codec
	mutex     sync.RWMutex
}

func (a *Adapter) ConsumerMessageToHttpRequest(ctx context.Context, span *trace.Span, cm *sarama.ConsumerMessage, req *nethttp.Request, logger *zap.Logger) error {
	msg := protocolkafka.NewMessageFromConsumerMessage(cm)

	defer func() {
		err := msg.Finish(nil)
		if err != nil {
			logger.Warn("Something went wrong while trying to finalizing the message", zap.Error(err))
		}
	}()

	// Build tracing ext to write it as output
	tracingExt := extensions.FromSpanContext(span.SpanContext())

	if msg.ReadEncoding() != binding.EncodingUnknown {
		// Message is a CloudEvent -> Encode directly to HTTP
		return http.WriteRequest(ctx, msg, req, tracingExt.WriteTransformer())
	}

	a.logger.Debug("Message is not a CloudEvent -> We need to translate it to a valid CloudEvent")
	kafkaMsg := msg

	if a.avroDecoder != nil {
		a.avroDecoder.decode(kafkaMsg, logger)
	}

	event := cloudevents.NewEvent()

	event.SetID(makeEventId(cm.Partition, cm.Offset))
	event.SetTime(cm.Timestamp)
	event.SetType(sourcesv1alpha1.KafkaEventType)
	event.SetSource(sourcesv1alpha1.KafkaEventSource(a.config.Namespace, a.config.Name, cm.Topic))
	event.SetSubject(makeEventSubject(cm.Partition, cm.Offset))

	dumpKafkaMetaToEvent(&event, a.keyTypeMapper, cm.Key, kafkaMsg)

	err := event.SetData(kafkaMsg.ContentType, kafkaMsg.Value)
	if err != nil {
		return err
	}

	return http.WriteRequest(ctx, binding.ToMessage(&event), req, tracingExt.WriteTransformer())
}

func makeEventId(partition int32, offset int64) string {
	var str strings.Builder
	str.WriteString("partition:")
	str.WriteString(strconv.Itoa(int(partition)))
	str.WriteString("/offset:")
	str.WriteString(strconv.FormatInt(offset, 10))
	return str.String()
}

// KafkaEventSubject returns the Kafka CloudEvent subject of the message.
func makeEventSubject(partition int32, offset int64) string {
	var str strings.Builder
	str.WriteString("partition:")
	str.WriteString(strconv.Itoa(int(partition)))
	str.WriteByte('#')
	str.WriteString(strconv.FormatInt(offset, 10))
	return str.String()
}

var replaceBadCharacters = regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString

func dumpKafkaMetaToEvent(event *cloudevents.Event, keyTypeMapper func([]byte) interface{}, key []byte, msg *protocolkafka.Message) {
	if key != nil && len(key) > 0 {
		event.SetExtension("key", keyTypeMapper(key))
	}
	for k, v := range msg.Headers {
		// Let's skip the content-type, we already transport it with datacontenttype field
		if k != "content-type" {
			event.SetExtension("kafkaheader"+replaceBadCharacters(k, ""), string(v))
		}
	}
}

func getKeyTypeMapper(keyType string) func([]byte) interface{} {
	var keyTypeMapper func([]byte) interface{}
	switch keyType {
	case "int":
		keyTypeMapper = func(by []byte) interface{} {
			// Took from https://github.com/axbaretto/kafka/blob/master/clients/src/main/java/org/apache/kafka/common/serialization/LongDeserializer.java
			if len(by) == 4 {
				var res int32
				for _, b := range by {
					res <<= 8
					res |= int32(b & 0xFF)
				}
				return res
			} else if len(by) == 8 {
				var res int64
				for _, b := range by {
					res <<= 8
					res |= int64(b & 0xFF)
				}
				return res
			} else {
				// Fallback to byte array
				return by
			}
		}
	case "float":
		keyTypeMapper = func(by []byte) interface{} {
			// BigEndian is specified in https://kafka.apache.org/protocol#protocol_types
			// Number is converted to string because
			if len(by) == 4 {
				intermediate := binary.BigEndian.Uint32(by)
				fl := math.Float32frombits(intermediate)
				return strconv.FormatFloat(float64(fl), 'f', -1, 64)
			} else if len(by) == 8 {
				intermediate := binary.BigEndian.Uint64(by)
				fl := math.Float64frombits(intermediate)
				return strconv.FormatFloat(fl, 'f', -1, 64)
			} else {
				// Fallback to byte array
				return by
			}
		}
	case "byte-array":
		keyTypeMapper = func(bytes []byte) interface{} {
			return bytes
		}
	default:
		keyTypeMapper = func(bytes []byte) interface{} {
			return string(bytes)
		}
	}
	return keyTypeMapper
}

func getAvroDecoder(schemaURL string) *AvroDecoder {
	if schemaURL != "" {
		return &AvroDecoder{
			schemaURL: schemaURL,
			codecMap:  make(map[int]*goavro.Codec),
		}
	}
	return nil
}

func (decoder *AvroDecoder) getCodec(schemaID int) (*goavro.Codec, error) {
	decoder.mutex.RLock()
	if decoder.codecMap[schemaID] != nil {
		return decoder.codecMap[schemaID], nil
	}
	decoder.mutex.RUnlock()

	resp, err := nethttp.Get(fmt.Sprintf("%s/schemas/ids/%d", decoder.schemaURL, schemaID))
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var schemaResp map[string]interface{}
	if err := json.Unmarshal(body, &schemaResp); err != nil {
		return nil, err
	}

	schema, err := json.Marshal(schemaResp["schema"])
	if err != nil {
		return nil, err
	}

	codec, err := goavro.NewCodec(string(schema))
	if err != nil {
		return nil, err
	}

	decoder.mutex.Lock()
	decoder.codecMap[schemaID] = codec
	decoder.mutex.Unlock()
	return codec, nil
}

func (decoder *AvroDecoder) decode(msg *protocolkafka.Message, logger *zap.Logger) {
	schemaID := binary.BigEndian.Uint32(msg.Value[1:5])
	if schemaID == 0 {
		logger.Warn("Error getting the schema id", zap.Int("schema id", int(schemaID)))
		return
	}
	codec, err := decoder.getCodec(int(schemaID))
	if err != nil {
		logger.Warn("Error getting the schema", zap.Error(err))
		return
	}
	native, _, err := codec.NativeFromBinary(msg.Value[5:])
	if err != nil {
		logger.Warn("Error decoding message", zap.Error(err))
		return
	}
	value, err := codec.TextualFromNative(nil, native)
	if err != nil {
		logger.Warn("Error decoding message", zap.Error(err))
		return
	}
	msg.Value = value
}
