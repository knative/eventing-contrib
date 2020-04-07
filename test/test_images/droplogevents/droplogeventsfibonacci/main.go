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

import "knative.dev/eventing-contrib/test/test_images/droplogevents"

func main() {
	droplogevents.StartReceiver(&fibonacci{
		prev:    1,
		current: 1,
	})
}

type fibonacci struct {
	prev    uint64
	current uint64
}

func (f *fibonacci) Skip(counter uint64) bool {
	if f.current == counter {
		f.Next()
		return true
	}
	return false
}

func (f *fibonacci) Next() {
	f.prev, f.current = f.current, f.prev+f.current
}
