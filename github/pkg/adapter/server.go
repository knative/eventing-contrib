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

package adapter

import (
	"context"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"knative.dev/eventing-contrib/github/pkg/client/listers/sources/v1alpha1"
)

// Context

type handlerKey struct{}

// WithHandler returns a copy of parent context in which the
// value associated with handler key is the supplied handler.
func WithHandler(ctx context.Context, handler *Handler) context.Context {
	return context.WithValue(ctx, handlerKey{}, handler)
}

// FromContext returns the handler stored in context.
// Returns nil if no handler is set in context, or if the stored value is
// not of correct type.
func HandlerFromContext(ctx context.Context) *Handler {
	if handler, ok := ctx.Value(handlerKey{}).(*Handler); ok {
		return handler
	}
	return nil
}

// Handler

type keyedHandler struct {
	handler   http.Handler
	namespace string
	name      string
}

// Handler holds the main GitHub webhook HTTP handler and delegate to sub-handlers
type Handler struct {
	log        *zap.SugaredLogger
	handlersMu sync.RWMutex
	handlers   map[string]keyedHandler
	lister     v1alpha1.GitHubSourceLister
}

// NewHandler create a new GitHub webhook handler receiving GitHub events
func NewHandler(logger *zap.SugaredLogger, lister v1alpha1.GitHubSourceLister) *Handler {
	return &Handler{
		log:      logger,
		handlers: make(map[string]keyedHandler),
		lister:   lister,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path-based dispatch
	h.handlersMu.RLock()
	keyedHandler, ok := h.handlers[r.URL.Path]
	h.handlersMu.RUnlock()

	if ok {
		// Still exists?
		_, err := h.lister.GitHubSources(keyedHandler.namespace).Get(keyedHandler.name)
		if err == nil {
			keyedHandler.handler.ServeHTTP(w, r)
			return

		}

		h.Unregister(r.URL.Path)
	}

	http.NotFoundHandler().ServeHTTP(w, r)
}

func (h *Handler) Register(name, namespace, path string, handler http.Handler) {
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()
	h.handlers[path] = keyedHandler{
		handler:   handler,
		namespace: namespace,
		name:      name,
	}
}

func (h *Handler) Unregister(path string) {
	h.handlersMu.Lock()
	defer h.handlersMu.Unlock()
	delete(h.handlers, path)
}

// NewServer creates a new http server receiving events from github
func NewServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
}
