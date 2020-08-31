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
package datacodec

import (
	protocolkafka "github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	"github.com/linkedin/goavro/v2"
	"go.uber.org/zap"
)

type AvroDecoder struct {
	codec *goavro.Codec
}

func GetAvroDecoder(schema string, logger *zap.SugaredLogger) *AvroDecoder {
	if schema != "" {
		c, err := goavro.NewCodec(schema)
		if err != nil {
			logger.Error("Unable to get codec from schema", zap.Error(err))
			return nil
		}
		return &AvroDecoder{
			codec: c,
		}
	}
	return nil
}

func (decoder *AvroDecoder) Decode(msg *protocolkafka.Message, logger *zap.SugaredLogger) ([]byte, error) {
	native, _, err := decoder.codec.NativeFromBinary(msg.Value[5:])
	if err != nil {
		logger.Error("Error decoding message", zap.Error(err))
		return nil, err
	}
	value, err := decoder.codec.TextualFromNative(nil, native)
	if err != nil {
		logger.Error("Error decoding message", zap.Error(err))
		return nil, err
	}
	return value, nil
}
