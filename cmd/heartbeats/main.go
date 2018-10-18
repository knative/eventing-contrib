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
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Heartbeat struct {
	Sequence int    `json:"id"`
	Label    string `json:"label"`
}

var (
	sink      string
	label     string
	periodStr string
	period    int
	sequence  int
	hb        *Heartbeat
)

func init() {
	flag.StringVar(&sink, "sink", "", "the host url to heartbeat to")
	flag.StringVar(&label, "label", "", "a special label")
	flag.StringVar(&periodStr, "period", "5", "the number of seconds between heartbeats")
}

func main() {
	flag.Parse()

	hb = &Heartbeat{
		Sequence: 0,
		Label:    label,
	}

	period, err := strconv.Atoi(periodStr)
	if err != nil {
		period = 5
	}

	for {
		send()
		time.Sleep(time.Duration(period) * time.Second)
	}
}

func send() {
	hb.Sequence++
	resp, err := http.Post(sink, "application/json", body())
	if err != nil {
		log.Printf("Unable to make request: %+v, %v", hb, err)
		return
	} else {
		log.Printf("[%d]: %+v", resp.StatusCode, hb)
	}
	defer resp.Body.Close()
}

func body() io.Reader {
	b, err := json.Marshal(hb)
	if err != nil {
		return strings.NewReader("{\"error\":\"true\"}")
	}
	return bytes.NewBuffer(b)
}
