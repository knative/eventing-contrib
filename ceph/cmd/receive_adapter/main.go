/*
Copyright 2019 The Knative Authors

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
	"log"

	"knative.dev/eventing-contrib/ceph/pkg/adapter"
)

func main() {
	sink := flag.String("sink", "", "uri to send events to")
	listenPort := flag.String("port", "8080", "listening port")

	flag.Parse()

	if sink == nil || *sink == "" {
		log.Fatalf("No sink given")
	}

	log.Printf("Sink is: %q. Listening on port: %q", *sink, *listenPort)

	// if successful, this should be a blocking call
	// ends only when the HTTP server exist
	adapter.Start(*sink, *listenPort)
}
