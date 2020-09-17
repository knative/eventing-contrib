/*
Copyright 2019 The Knative Authors.

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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

const (
	// CephConditionReady has status True when the CephSource is ready to send events.
	CephConditionReady = apis.ConditionReady

	// CephConditionSinkProvided has status True when the CephSource has been configured with a sink target.
	CephConditionSinkProvided apis.ConditionType = "SinkProvided"

	// CephConditionDeployed has status True when the CephSource has had it's deployment created.
	CephConditionDeployed apis.ConditionType = "Deployed"
)

var CephCondSet = apis.NewLivingConditionSet(
	CephConditionSinkProvided,
	CephConditionDeployed,
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CephSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return CephCondSet.Manage(s).GetCondition(t)
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CephSourceStatus) InitializeConditions() {
	CephCondSet.Manage(s).InitializeConditions()
}

// GetConditionSet returns CephSource ConditionSet.
func (*CephSource) GetConditionSet() apis.ConditionSet {
	return CephCondSet
}

// MarkSink sets the condition that the source has a sink configured.
func (s *CephSourceStatus) MarkSink(uri *apis.URL) {
	s.SinkURI = uri
	if len(uri.String()) > 0 {
		CephCondSet.Manage(s).MarkTrue(CephConditionSinkProvided)
	} else {
		CephCondSet.Manage(s).MarkUnknown(CephConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CephSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	CephCondSet.Manage(s).MarkFalse(CephConditionSinkProvided, reason, messageFormat, messageA...)
}

// PropagateDeploymentAvailability uses the availability of the provided Deployment to determine if
// CephConditionDeployed should be marked as true or false.
func (s *CephSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		CephCondSet.Manage(s).MarkTrue(CephConditionDeployed)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		CephCondSet.Manage(s).MarkFalse(CephConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// IsReady returns true if the resource is ready overall.
func (s *CephSourceStatus) IsReady() bool {
	return CephCondSet.Manage(s).IsHappy()
}
