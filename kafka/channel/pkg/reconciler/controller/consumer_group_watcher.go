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
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-contrib/kafka/channel/pkg/kafka"
)

var (
	watchersMtx sync.RWMutex
	cacheMtx    sync.RWMutex
	// Hooks into the poll logic for testing
	after = time.After
	done  = func() {}
)

type ConsumerGroupHandler func()
type Matcher func(string) bool

type ConsumerGroupWatcher interface {
	// Start instructs the watcher to start polling for the consumer groups and
	// notify any observers on the event of any changes
	Start() error

	// Terminate instructs the watcher to stop polling and clear the watchers cache
	Terminate()

	// Watch registers callback on the event of any changes observed
	// on the consumer groups. watcherID is an arbitrary string the user provides
	// that will be used to identify his callbacks when provided to Forget(watcherID).
	//
	// To ensure this is event-triggered, level-driven,
	// we don't pass the updates to the callback, instead the observer is expected
	// to use List() to get the updated list of ConsumerGroups.
	Watch(watcherID string, callback ConsumerGroupHandler) error

	// Forget removes all callbacks that correspond to the watcherID
	Forget(watcherID string)

	// List returns all the cached consumer groups that match matcher.
	// It will return an empty slice if none matched or the cache is empty
	List(matcher Matcher) []string
}

type WatcherImpl struct {
	logger *zap.SugaredLogger
	//TODO name?
	watchers             map[string]ConsumerGroupHandler
	cachedConsumerGroups sets.String
	adminClient          kafka.AdminClient
	pollDuration         time.Duration
	done                 chan struct{}
}

func NewConsumerGroupWatcher(ctx context.Context, ac kafka.AdminClient, pollDuration time.Duration) ConsumerGroupWatcher {
	return &WatcherImpl{
		logger:               logging.FromContext(ctx),
		adminClient:          ac,
		pollDuration:         pollDuration,
		watchers:             make(map[string]ConsumerGroupHandler),
		cachedConsumerGroups: sets.String{},
	}
}

func (w *WatcherImpl) Start() error {
	w.logger.Infow("ConsumerGroupWatcher starting. Polling for consumer groups", zap.Duration("poll duration", w.pollDuration))
	go func() {
		for {
			select {
			case <-after(w.pollDuration):
				// let's get current observed consumer groups
				observedCGs, err := w.adminClient.ListConsumerGroups()
				if err != nil {
					w.logger.Errorw("error while listing consumer groups", zap.Error(err))
					continue
				}
				var notify bool
				var changedGroup string
				observedCGsSet := sets.String{}.Insert(observedCGs...)
				// Look for observed CGs
				for c := range observedCGsSet {
					if !w.cachedConsumerGroups.Has(c) {
						// This is the first appearance.
						w.logger.Debugw("Consumer group observed. Caching.",
							zap.String("consumer group", c))
						changedGroup = c
						notify = true
						break
					}
				}
				// Look for disappeared CGs
				for c := range w.cachedConsumerGroups {
					if !observedCGsSet.Has(c) {
						// This CG was cached but it's no longer there.
						w.logger.Debugw("Consumer group deleted.",
							zap.String("consumer group", c))
						changedGroup = c
						notify = true
						break
					}
				}
				if notify {
					cacheMtx.Lock()
					w.cachedConsumerGroups = observedCGsSet
					cacheMtx.Unlock()
					w.notify(changedGroup)
				}
				done()
			case <-w.done:
				break
			}
		}
	}()
	return nil
}

func (w *WatcherImpl) Terminate() {
	watchersMtx.Lock()
	cacheMtx.Lock()
	defer watchersMtx.Unlock()
	defer cacheMtx.Unlock()

	w.watchers = nil
	w.cachedConsumerGroups = nil
	w.done <- struct{}{}
}

// TODO explore returning a channel instead of a taking callback
func (w *WatcherImpl) Watch(watcherID string, cb ConsumerGroupHandler) error {
	w.logger.Debugw("Adding a new watcher", zap.String("watcherID", watcherID))
	watchersMtx.Lock()
	defer watchersMtx.Unlock()
	w.watchers[watcherID] = cb

	// notify at least once to get the current state
	cb()
	return nil
}

func (w *WatcherImpl) Forget(watcherID string) {
	w.logger.Debugw("Forgetting watcher", zap.String("watcherID", watcherID))
	watchersMtx.Lock()
	defer watchersMtx.Unlock()
	delete(w.watchers, watcherID)
}

func (w *WatcherImpl) List(matcher Matcher) []string {
	w.logger.Debug("Listing consumer groups")
	cacheMtx.RLock()
	defer cacheMtx.RUnlock()
	cgs := make([]string, 0)
	for cg := range w.cachedConsumerGroups {
		if matcher(cg) {
			cgs = append(cgs, cg)
		}
	}
	return cgs
}

func (w *WatcherImpl) notify(cg string) {
	watchersMtx.RLock()
	defer watchersMtx.RUnlock()

	for _, cb := range w.watchers {
		cb()
	}
}

func Find(list []string, item string) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}
