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
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/Shopify/sarama"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/tracestate"
)

const (
	supportedVersion  = 0
	traceParentHeader = "traceparent"
	traceStateHeader  = "tracestate"
	maxVersion        = 254
	maxTracestateLen  = 512
	trimOWSRegexFmt   = `^[\x09\x20]*(.*[^\x20\x09])[\x09\x20]*$`
)

var trimOWSRegExp = regexp.MustCompile(trimOWSRegexFmt)

// Part of this code is took from https://github.com/census-instrumentation/opencensus-go/blob/master/plugin/ochttp/propagation/tracecontext/propagation.go
// Apache License 2.0
// TODO(slinkydeveloper) remove this code as soon as https://github.com/census-instrumentation/opencensus-go/pull/1218 is merged

func serializeTrace(spanContext trace.SpanContext) []sarama.RecordHeader {
	var headers []sarama.RecordHeader

	traceparent := fmt.Sprintf("%x-%x-%x-%x",
		[]byte{supportedVersion},
		spanContext.TraceID[:],
		spanContext.SpanID[:],
		[]byte{byte(spanContext.TraceOptions)},
	)
	headers = append(headers, sarama.RecordHeader{Key: []byte(traceParentHeader), Value: []byte(traceparent)})

	if len(spanContext.Tracestate.Entries()) != 0 {
		entries := make([]string, 0, len(spanContext.Tracestate.Entries()))
		for _, entry := range spanContext.Tracestate.Entries() {
			entries = append(entries, strings.Join([]string{entry.Key, entry.Value}, "="))
		}
		tracestate := strings.Join(entries, ",")

		headers = append(headers, sarama.RecordHeader{Key: []byte(traceStateHeader), Value: []byte(tracestate)})
	}

	return headers
}

func parseSpanContext(headers map[string][]byte) (sc trace.SpanContext, ok bool) {
	hBytes, ok := headers[traceParentHeader]
	if !ok {
		return trace.SpanContext{}, false
	}
	h := string(hBytes)

	sections := strings.Split(h, "-")
	if len(sections) < 4 {
		return trace.SpanContext{}, false
	}

	if len(sections[0]) != 2 {
		return trace.SpanContext{}, false
	}
	ver, err := hex.DecodeString(sections[0])
	if err != nil {
		return trace.SpanContext{}, false
	}
	version := int(ver[0])
	if version > maxVersion {
		return trace.SpanContext{}, false
	}

	if version == 0 && len(sections) != 4 {
		return trace.SpanContext{}, false
	}

	if len(sections[1]) != 32 {
		return trace.SpanContext{}, false
	}
	tid, err := hex.DecodeString(sections[1])
	if err != nil {
		return trace.SpanContext{}, false
	}
	copy(sc.TraceID[:], tid)

	if len(sections[2]) != 16 {
		return trace.SpanContext{}, false
	}
	sid, err := hex.DecodeString(sections[2])
	if err != nil {
		return trace.SpanContext{}, false
	}
	copy(sc.SpanID[:], sid)

	opts, err := hex.DecodeString(sections[3])
	if err != nil || len(opts) < 1 {
		return trace.SpanContext{}, false
	}
	sc.TraceOptions = trace.TraceOptions(opts[0])

	// Don't allow all zero trace or span ID.
	if sc.TraceID == [16]byte{} || sc.SpanID == [8]byte{} {
		return trace.SpanContext{}, false
	}

	sc.Tracestate = tracestateFromRequest(headers)
	return sc, true
}

func tracestateFromRequest(headers map[string][]byte) *tracestate.Tracestate {
	hBytes, ok := headers[traceStateHeader]
	if !ok {
		return nil
	}
	h := string(hBytes)

	var entries []tracestate.Entry
	pairs := strings.Split(h, ",")
	hdrLenWithoutOWS := len(pairs) - 1 // Number of commas
	for _, pair := range pairs {
		matches := trimOWSRegExp.FindStringSubmatch(pair)
		if matches == nil {
			return nil
		}
		pair = matches[1]
		hdrLenWithoutOWS += len(pair)
		if hdrLenWithoutOWS > maxTracestateLen {
			return nil
		}
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return nil
		}
		entries = append(entries, tracestate.Entry{Key: kv[0], Value: kv[1]})
	}
	ts, err := tracestate.New(nil, entries...)
	if err != nil {
		return nil
	}

	return ts
}
