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
	"context"
	"flag"
	"os"

	"knative.dev/pkg/configmap"
	kncontroller "knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"

	"knative.dev/eventing-contrib/natss/pkg/reconciler/controller"
)

const component = "natsschannel_controller"

func main() {
	flag.Parse()
	ctx := signals.NewContext()
	ns := os.Getenv("NAMESPACE")
	if ns != "" {
		ctx = injection.WithNamespaceScope(ctx, ns)
	}

	cfg := sharedmain.ParseAndGetConfigOrDie()
	sharedmain.MainWithConfig(ctx, component, cfg, func(ctx context.Context, watcher configmap.Watcher) *kncontroller.Impl {
		return controller.NewController(ctx, watcher, cfg)
	})
}
