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

package kafka

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Shopify/sarama"

	pkgtesting "knative.dev/pkg/logging/testing"
)

const testCG = "cg1"

var m sync.RWMutex

type FakeClusterAdmin struct {
	sarama.ClusterAdmin
	faulty bool
}

func (f *FakeClusterAdmin) ListConsumerGroups() (map[string]string, error) {
	cgs := map[string]string{
		testCG: "cg",
	}
	m.RLock()
	defer m.RUnlock()
	if f.faulty {
		return nil, fmt.Errorf("Error")
	}
	return cgs, nil
}

func TestAdminClient(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(10)
	ctx := pkgtesting.TestContextWithLogger(t)
	f := &FakeClusterAdmin{}
	ac, err := NewAdminClient(ctx, func() (sarama.ClusterAdmin, error) {
		return f, nil
	})
	if err != nil {
		t.Error("failed to obtain new client", err)
	}
	for i := 0; i < 10; i += 1 {
		go func() {
			doList(t, ac)
			check := make(chan struct{})
			go func() {
				m.Lock()
				f.faulty = true
				m.Unlock()
				check <- struct{}{}
				time.Sleep(2 * time.Second)
				m.Lock()
				f.faulty = false
				m.Unlock()
				check <- struct{}{}
			}()
			<-check
			doList(t, ac)
			<-check
			wg.Done()
		}()
	}
	wg.Wait()
}

func doList(t *testing.T, ac AdminClient) {
	cgs, _ := ac.ListConsumerGroups()
	if len(cgs) != 1 {
		t.Fatalf("list consumer group: got %d, want %d", len(cgs), 1)
	}
	if cgs[0] != testCG {
		t.Fatalf("consumer group: got %s, want %s", cgs[0], testCG)
	}
}
