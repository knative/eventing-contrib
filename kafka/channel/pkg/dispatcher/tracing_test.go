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
)

func TestRoundtrip(t *testing.T) {
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
