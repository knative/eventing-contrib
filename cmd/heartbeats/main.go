/*
Copyright 2018 The Knative Authors

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

package main

import (
	"flag"
	"strconv"
	"time"

	log "github.com/mgutz/logxi/v1"

	"github.com/knative/pkg/cloudevents"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Label    string `json:"label"`
}

var (
	sink      string
	label     string
	periodStr string
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to heartbeat to")
	flag.StringVar(&label, "label", "", "a special label")
	flag.StringVar(&periodStr, "period", "5", "the number of seconds between heartbeats")
}

func main() {
	flag.Parse()

	client := cloudevents.NewClient(sink, cloudevents.Builder{
		EventType: "dev.knative.source.heartbeats",
		Source:    "heartbeats-demo",
	})

	var period time.Duration
	if p, err := strconv.Atoi(periodStr); err != nil {
		period = time.Duration(5) * time.Second
	} else {
		period = time.Duration(p) * time.Second
	}

	hb := &Heartbeat{
		Sequence: 0,
		Label:    label,
	}
	ticker := time.NewTicker(period)
	for {
		hb.Sequence++
		if err := client.Send(hb); err != nil {
			log.Error("failed to send cloudevent", err)
		}
		// Wait for next tick
		<-ticker.C
	}
}
