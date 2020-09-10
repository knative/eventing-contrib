/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"log"

	"knative.dev/pkg/controller"

	"knative.dev/pkg/signals"

	githubadapter "knative.dev/eventing-contrib/github/pkg/mtadapter"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
)

func main() {
	cfg := sharedmain.ParseAndGetConfigOrDie()

	ctx := signals.NewContext()
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	log.Println("Starting informers...")
	if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
		log.Fatalf("Failed to start informers: %v", err)
	}

	adapter.MainWithContext(ctx, "githubsource", githubadapter.NewEnvConfig, githubadapter.NewAdapter)
}
