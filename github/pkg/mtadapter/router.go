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
	"errors"
	"net/http"
	"sync"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"

	"knative.dev/eventing-contrib/github/pkg/client/listers/sources/v1alpha1"
	"knative.dev/eventing-contrib/github/pkg/common"
)

type keyedHandler struct {
	handler   http.Handler
	namespace string
	name      string
}

// Router holds the main GitHub webhook HTTP router and delegate to sub-routers
type Router struct {
	logger           *zap.SugaredLogger
	routersMu        sync.RWMutex
	routers          map[string]keyedHandler
	lister           v1alpha1.GitHubSourceLister
	ceClient         cloudevents.Client
	vanityURLEnabled bool
}

// NewRouter create a new GitHub webhook router receiving GitHub events
func NewRouter(logger *zap.SugaredLogger, lister v1alpha1.GitHubSourceLister, ceClient cloudevents.Client) *Router {
	return &Router{
		logger:           logger,
		routers:          make(map[string]keyedHandler),
		lister:           lister,
		ceClient:         ceClient,
		vanityURLEnabled: false,
	}
}

// EnableVanityURL tells the router the Github controller generates
// vanity URL and extends the HTTP headers with information about
// the original CR
func (h *Router) EnableVanityURL() {
	h.vanityURLEnabled = true
}

func (h *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, err := h.dispatchKey(r)
	if err != nil {
		BadRequestHandler(err.Error()).ServeHTTP(w, r)
		return
	}
	h.logger.Info("received request from github", zap.String("key", key))

	// Path-based dispatch
	h.routersMu.RLock()
	keyedHandler, ok := h.routers[key]
	h.routersMu.RUnlock()

	if ok {
		// Check if source still exists.
		_, err := h.lister.GitHubSources(keyedHandler.namespace).Get(keyedHandler.name)
		if err == nil {
			keyedHandler.handler.ServeHTTP(w, r)
			return
		}

		h.Unregister(key)
	}
	http.NotFoundHandler().ServeHTTP(w, r)
}

// Register adds a new Github event handler for the given GitHubSource
func (h *Router) Register(name, namespace, path string, handler http.Handler) {
	h.logger.Info("register githubsource", zap.String("key", path))
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
	h.logger.Info("unregister githubsource", zap.String("key", path))
	h.routersMu.Lock()
	defer h.routersMu.Unlock()
	delete(h.routers, path)
}

func (h *Router) dispatchKey(r *http.Request) (string, error) {
	if h.vanityURLEnabled {
		name := r.Header.Get(common.HeaderName)
		if name == "" {
			return "", errors.New("missing HTTP header 'name'")
		}
		ns := r.Header.Get(common.HeaderNamespace)
		if ns == "" {
			return "", errors.New("missing HTTP header 'namespace'")
		}
		return "/" + ns + "/" + name, nil
	}

	return r.URL.Path, nil
}

// BadRequest replies to the request with an HTTP 400 bad request error.
func BadRequest(msg string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, msg, http.StatusBadRequest)
	}
}

// BadRequestHandler returns a simple request handler
// that replies to each request with a ``400 bad request'' reply.
func BadRequestHandler(msg string) http.Handler { return http.HandlerFunc(BadRequest(msg)) }
