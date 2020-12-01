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

package controller

import (
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	pkgtesting "knative.dev/pkg/logging/testing"
)

//TODO how to mock the sarama AdminClient
type FakeClusterAdmin struct {
	mutex sync.RWMutex
	cgs   sets.String
}

func (fake *FakeClusterAdmin) ListConsumerGroups() ([]string, error) {
	fake.mutex.RLock()
	defer fake.mutex.RUnlock()
	return fake.cgs.List(), nil
}

func (fake *FakeClusterAdmin) deleteCG(cg string) {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	fake.cgs.Delete(cg)
}

func TestKafkaWatcher(t *testing.T) {
	cgname := "kafka.event-example.default-kne-trigger.0d9c4383-1e68-42b5-8c3a-3788274404c5"
	wid := "channel-abc"
	cgs := sets.String{}
	cgs.Insert(cgname)
	ca := FakeClusterAdmin{
		cgs: cgs,
	}

	ch := make(chan sets.String, 1)

	w := NewConsumerGroupWatcher(pkgtesting.TestContextWithLogger(t), &ca, 2*time.Second)
	w.Watch(wid, func() {
		cgs := w.List(func(cg string) bool {
			return cgname == cg
		})
		result := sets.String{}
		result.Insert(cgs...)
		ch <- result
	})

	w.Start()
	<-ch
	assertSync(t, ch, cgs)
	ca.deleteCG(cgname)
	assertSync(t, ch, sets.String{})
}

func assertSync(t *testing.T, ch chan sets.String, cgs sets.String) {
	select {
	case syncedCGs := <-ch:
		if !syncedCGs.Equal(cgs) {
			t.Errorf("observed and expected consumer groups do not match. got %v expected %v", syncedCGs, cgs)
		}
	case <-time.After(6 * time.Second):
		t.Errorf("timedout waiting for consumer groups to sync")
	}
}
