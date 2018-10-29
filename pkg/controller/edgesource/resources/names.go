/*
Copyright 2018 The Knative Authors

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

package resources

import (
	"fmt"
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
)

// JobName returns the name of the job for the source, taking into account
// whether or not it has been deleted.
func JobName(s *v1alpha1.EdgeSource, job *v1alpha1.JobSpec) string {
	if job.Type == v1alpha1.EdgeJobStart {
		return StartJobName(s)
	}
	return StopJobName(s)
}

// StartJobName returns the name of the job for the start operation.
func StartJobName(s *v1alpha1.EdgeSource) string {
	return fmt.Sprintf("%s-start-", s.Name)
}

// StopJobName returns the name of the job for the stop operation.
func StopJobName(s *v1alpha1.EdgeSource) string {
	return fmt.Sprintf("%s-stop-", s.Name)
}
