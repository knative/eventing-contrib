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

package test

import (
	"github.com/knative/serving/pkg/apis/serving/v1beta1"
)

// states contains functions for asserting against the state of Knative Serving
// crds to see if they have achieved the states specified in the spec
// (https://github.com/knative/serving/blob/master/docs/spec/spec.md).

// IsRevisionReady will check the status conditions of the revision and return true if the revision is
// ready to serve traffic. It will return false if the status indicates a state other than deploying
// or being ready. It will also return false if the type of the condition is unexpected.
func IsRevisionReady(r *v1beta1.Revision) (bool, error) {
	return r.Status.IsReady(), nil
}

// IsServiceReady will check the status conditions of the service and return true if the service is
// ready. This means that its configurations and routes have all reported ready.
func IsServiceReady(s *v1beta1.Service) (bool, error) {
	return s.Status.IsReady(), nil
}

// IsRouteReady will check the status conditions of the route and return true if the route is
// ready.
func IsRouteReady(r *v1beta1.Route) (bool, error) {
	return r.Status.IsReady(), nil
}
