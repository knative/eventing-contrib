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
package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKafkaRequestFactory(t *testing.T) {
	const sizeRandomPayload = 100
	const expectedMessageType = "testmessagetype"
	const expectedUuid = "7d446840-9dc0-11d1-b245-5ffdce74fad2"
	// Note, these aren't currently encoded
	const inputID = 90
	inputTime := time.Unix(1600000000, 0)

	for _, fixedPayload := range []bool{false, true} {
		requestFactory := JsonKafkaRequestFactory(sizeRandomPayload, expectedMessageType, fixedPayload)
		request := requestFactory(inputTime, inputID, expectedUuid)
		var decode map[string]interface{}

		err := json.Unmarshal(request, &decode)
		if err != nil {
			t.Fatalf("Error json decoding request: %s", err)
		}
		nonEmptyStringFields := []string{"id", "type", "randomStuff"}
		for _, field := range nonEmptyStringFields {
			val, found := decode[field]
			if !found {
				t.Fatalf("Expected request field missing: %s", field)
			}
			str, ok := val.(string)
			if !ok {
				t.Fatalf("Expected request field (%s) of wrong type: %+v", field, val)
			}
			if len(str) == 0 {
				t.Fatalf("Request field unexpectedly the empty string: %s", field)
			}
		}

		id := (decode["id"]).(string)
		if id != expectedUuid {
			t.Errorf("Unexpected id in decode = %s, want %s", id, expectedUuid)
		}
		typeDec := (decode["type"]).(string)
		if typeDec != expectedMessageType {
			t.Errorf("Unexpected type in decode = %s, want %s", typeDec, expectedMessageType)
		}
		randomString1 := (decode["randomStuff"]).(string)
		if len(randomString1) != sizeRandomPayload {
			t.Errorf("Unexpected length of random payload = %d, want %d", len(randomString1), sizeRandomPayload)
		}

		request2 := requestFactory(inputTime, inputID, expectedUuid)
		var decode2 map[string]interface{}

		err = json.Unmarshal(request2, &decode2)
		if err != nil {
			t.Fatalf("Error json decoding request: %s", err)
		}
		val, ok := decode2["randomStuff"]
		if !ok {
			t.Fatal("Expected request field randomStuff missing")
		}

		randomString2, ok := val.(string)
		if !ok {
			t.Fatalf("Request field randomStuff of wrong type: %#v", val)
		}
		if fixedPayload && randomString2 != randomString1 {
			t.Errorf("Unexpected mismatch of random payload = (%s), want (%s)", randomString2, randomString1)
		} else if !fixedPayload && randomString2 == randomString1 {
			t.Errorf("Unexpected match of random payload")
		}

	}
}

func BenchmarkRequestFixedPayload(b *testing.B) {
	const sizeRandomPayload = 100
	const messageType = "testmessagetype"
	const uuid = "7d446840-9dc0-11d1-b245-5ffdce74fad2"

	// Note, these aren't currently encoded
	const inputID = 90
	inputTime := time.Unix(1600000000, 0)

	requestFactory := JsonKafkaRequestFactory(sizeRandomPayload, messageType, true)

	for i := 0; i < b.N; i++ {
		request := requestFactory(inputTime, sizeRandomPayload, uuid)
		_ = request
	}
}

func BenchmarkRequestRandomPayload(b *testing.B) {
	const sizeRandomPayload = 100
	const messageType = "testmessagetype"
	const uuid = "7d446840-9dc0-11d1-b245-5ffdce74fad2"

	// Note, these aren't currently encoded
	const inputID = 90
	inputTime := time.Unix(1600000000, 0)

	requestFactory := JsonKafkaRequestFactory(sizeRandomPayload, messageType, false)

	for i := 0; i < b.N; i++ {
		request := requestFactory(inputTime, sizeRandomPayload, uuid)
		_ = request
	}
}
