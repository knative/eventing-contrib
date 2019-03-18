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

// This file contains logic to encapsulate flags which are needed to specify
// what cluster, etc. to use for e2e tests.

package test

import (
	"flag"
	"os"

	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

// EventingSourcesFlags holds the command line flags specific to knative/eventing-sources
var EventingSourcesFlags = initializeEventingSourcesFlags()

// EventingSourcesEnvironmentFlags holds the e2e flags needed only by the eventing-sources repo
type EventingSourcesEnvironmentFlags struct {
	DockerRepo string // Docker repo (defaults to $KO_DOCKER_REPO)
	Tag        string // Tag for test images
}

func initializeEventingSourcesFlags() *EventingSourcesEnvironmentFlags {
	var f EventingSourcesEnvironmentFlags

	defaultRepo := os.Getenv("KO_DOCKER_REPO")
	flag.StringVar(&f.DockerRepo, "dockerrepo", defaultRepo,
		"Provide the uri of the docker repo you have uploaded the test image to using `uploadtestimage.sh`. Defaults to $KO_DOCKER_REPO")

	flag.StringVar(&f.Tag, "tag", "e2e", "Provide the version tag for the test images.")

	flag.Parse()

	logging.InitializeLogger(pkgTest.Flags.LogVerbose)

	if pkgTest.Flags.EmitMetrics {
		logging.InitializeMetricExporter("eventing-sources")
	}

	return &f
}
