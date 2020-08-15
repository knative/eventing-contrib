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

package dispatcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/tracestate"
)

var (
	traceID             = trace.TraceID{75, 249, 47, 53, 119, 179, 77, 166, 163, 206, 146, 157, 14, 14, 71, 54}
	spanID              = trace.SpanID{0, 240, 103, 170, 11, 169, 2, 183}
	traceOpt            = trace.TraceOptions(1)
	entry1              = tracestate.Entry{Key: "foo", Value: "bar"}
	entry2              = tracestate.Entry{Key: "hello", Value: "world   example"}
	sampleTracestate, _ = tracestate.New(nil, entry1, entry2)
	sampleSpanContext   = trace.SpanContext{
		TraceID:      traceID,
		SpanID:       spanID,
		TraceOptions: traceOpt,
		Tracestate:   sampleTracestate,
	}
)

func TestRoundtripWithNewSpan(t *testing.T) {
	_, span := trace.StartSpan(context.TODO(), "aaa", trace.WithSpanKind(trace.SpanKindClient))
	span.AddAttributes(trace.BoolAttribute("hello", true))
	inSpanContext := span.SpanContext()

	serializedHeaders := serializeTrace(inSpanContext)

	// Translate back to headers
	headers := make(map[string][]byte)
	for _, h := range serializedHeaders {
		headers[string(h.Key)] = h.Value
	}

	outSpanContext, ok := parseSpanContext(headers)
	require.True(t, ok)
	require.Equal(t, inSpanContext, outSpanContext)
}

func TestRoundtrip(t *testing.T) {
	serializedHeaders := serializeTrace(sampleSpanContext)

	// Translate back to headers
	headers := make(map[string][]byte)
	for _, h := range serializedHeaders {
		headers[string(h.Key)] = h.Value
	}

	outSpanContext, ok := parseSpanContext(headers)
	require.True(t, ok)
	require.Equal(t, sampleSpanContext, outSpanContext)
}
