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
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"

	"github.com/google/go-cmp/cmp"

	cloudevents "github.com/cloudevents/sdk-go"

	stan "github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/go-nats-streaming/pb"
)

func TestSerialize(t *testing.T) {

	tt := []struct {
		name  string
		event *cloudevents.Event
		want  []byte
	}{
		{
			name: "event with extension",
			event: &cloudevents.Event{
				Context: &cloudevents.EventContextV1{
					ID:              "af0a8168-66c0-4d67-a28f-23a55ad8ade3",
					Source:          *cloudevents.ParseURIRef("https://github.com/knative"),
					SpecVersion:     "1.0",
					Type:            "com.example.someevent",
					DataContentType: getStringRef("application/json"),
					Time:            getTimestampRef("2018-04-05T17:31:00Z"),
					Extensions:      map[string]interface{}{"app1": "app1"},
				},
				Data: map[string]interface{}{"dk1": "dv1", "dk2": "dv2"},
			},
			want: toJson(map[string]interface{}{
				"id":              "af0a8168-66c0-4d67-a28f-23a55ad8ade3",
				"source":          "https://github.com/knative",
				"specversion":     "1.0",
				"type":            "com.example.someevent",
				"datacontenttype": "application/json",
				"time":            "2018-04-05T17:31:00Z",
				"app1":            "app1",
				"data":            map[string]interface{}{"dk1": "dv1", "dk2": "dv2"},
			}),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			b, err := serialize(tc.event)
			if err != nil {
				t.Errorf("unexpected error %+v", err)
			}
			if diff := cmp.Diff(b, tc.want); diff != "" {
				t.Errorf("expected json %s got %s\ndiff: %s", string(tc.want), b, diff)
			}
		})
	}
}

func TestToEvent(t *testing.T) {

	tt := []struct {
		name          string
		msg           *stan.Msg
		want          *cloudevents.Event
		errorExpected bool
	}{
		{
			name: "invalid extension",
			msg: &stan.Msg{
				MsgProto: pb.MsgProto{
					Subject: "subject-1",
					Data: []byte(`{
						"specversion": "1.0",
						"type": "com.example.someevent",
						"id": "af0a8168-66c0-4d67-a28f-23a55ad8ade3",
						"source": "https://github.com/knative",
						"datacontenttype": "application/json",
						"time": "2018-04-05T17:31:00Z",
						"comexampleextension1": "value-extension-1",
						"invalid_extension": "value-extension-2",
						"data": {
							"app1": "app1"
						}}`),
				},
			},
			want: func(t *testing.T) *cloudevents.Event {
				e := cloudevents.NewEvent(cloudEventVersion)
				e.SetSubject("subject-1")
				e.SetType("com.example.someevent")
				e.SetID("af0a8168-66c0-4d67-a28f-23a55ad8ade3")
				e.SetSource("https://github.com/knative")
				e.SetDataContentType("application/json")
				time, _ := time.Parse(timeLayout, "2018-04-05T17:31:00Z")
				e.SetTime(time)
				e.SetExtension("comexampleextension1", "value-extension-1")
				if err := e.SetData(map[string]interface{}{"app1": "app1"}); err != nil {
					t.Errorf("failed to set data: %+v", err)
				}
				return &e
			}(t),
		},
		{
			name: "invalid data",
			msg: &stan.Msg{
				MsgProto: pb.MsgProto{
					Data: []byte("data"),
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			e, err := toEvent(tc.msg)
			if tc.errorExpected != (err != nil) {
				t.Errorf("expected error %+v", err)
			}
			if diff := cmp.Diff(e, tc.want); diff != "" {
				t.Errorf("expected event %+v got %+v\ndiff: %+v", tc.want, e, diff)
			}
		})
	}
}

func getStringRef(s string) *string {
	return &s
}

func getTimestampRef(tStr string) *types.Timestamp {
	t, _ := types.ParseTimestamp(tStr)
	return t
}

func toJson(j map[string]interface{}) []byte {
	data, _ := json.Marshal(j)
	return data
}
