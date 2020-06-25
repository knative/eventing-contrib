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

package mtadapter

import (
	"net/http"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/eventing-contrib/github/pkg/client/listers/sources/v1alpha1"
)

type keyedHandler struct {
	handler   http.Handler
	namespace string
	name      string
}

// Router holds the main GitHub webhook HTTP router and delegate to sub-routers
type Router struct {
	logger    *zap.SugaredLogger
	routersMu sync.RWMutex
	routers   map[string]keyedHandler
	lister    v1alpha1.GitHubSourceLister
	ceClient  cloudevents.Client
}

// NewRouter create a new GitHub webhook router receiving GitHub events
func NewRouter(logger *zap.SugaredLogger, lister v1alpha1.GitHubSourceLister, ceClient cloudevents.Client) *Router {
	return &Router{
		logger:   logger,
		routers:  make(map[string]keyedHandler),
		lister:   lister,
		ceClient: ceClient,
	}
}

func (h *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path-based dispatch
	h.routersMu.RLock()
	keyedHandler, ok := h.routers[r.URL.Path]
	h.routersMu.RUnlock()

	if ok {
		// Check if source still exists.
		_, err := h.lister.GitHubSources(keyedHandler.namespace).Get(keyedHandler.name)
		if err == nil {
			keyedHandler.handler.ServeHTTP(w, r)
			return
		}

		h.Unregister(r.URL.Path)
	}
	http.NotFoundHandler().ServeHTTP(w, r)
}

// Register adds a new Github event handler for the given GitHubSource
func (h *Router) Register(name, namespace, path string, handler http.Handler) {
	h.routersMu.Lock()
	defer h.routersMu.Unlock()
	h.routers[path] = keyedHandler{
		handler:   handler,
		namespace: namespace,
		name:      name,
	}
}

// Unregister removes the GitHubSource served at the given path
func (h *Router) Unregister(path string) {
	h.routersMu.Lock()
	defer h.routersMu.Unlock()
	delete(h.routers, path)
}
