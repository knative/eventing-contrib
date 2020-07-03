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
