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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

const (
	// PrometheusConditionReady has status True when the PrometheusSource is ready to send events.
	PrometheusConditionReady = apis.ConditionReady

	// PrometheusConditionValidSchedule has status True when the PrometheusSource has been configured with a valid schedule.
	PrometheusConditionValidSchedule apis.ConditionType = "ValidSchedule"

	// PrometheusConditionSinkProvided has status True when the PrometheusSource has been configured with a sink target.
	PrometheusConditionSinkProvided apis.ConditionType = "SinkProvided"

	// PrometheusConditionDeployed has status True when the PrometheusSource has had it's deployment created.
	PrometheusConditionDeployed apis.ConditionType = "Deployed"

	// PrometheusConditionEventTypeProvided has status True when the PrometheusSource has been configured with its event types.
	PrometheusConditionEventTypeProvided apis.ConditionType = "EventTypesProvided"
)

var PrometheusCondSet = apis.NewLivingConditionSet(
	PrometheusConditionSinkProvided,
	PrometheusConditionDeployed,
	PrometheusConditionEventTypeProvided,
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *PrometheusSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return PrometheusCondSet.Manage(s).GetCondition(t)
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *PrometheusSourceStatus) InitializeConditions() {
	PrometheusCondSet.Manage(s).InitializeConditions()
}

// MarkValidSchedule sets the condition that the source has a valid schedule configured.
func (s *PrometheusSourceStatus) MarkValidSchedule() {
	PrometheusCondSet.Manage(s).MarkTrue(PrometheusConditionValidSchedule)
}

// MarkInvalidSchedule sets the condition that the source does not have a valid schedule configured.
func (s *PrometheusSourceStatus) MarkInvalidSchedule(reason, messageFormat string, messageA ...interface{}) {
	PrometheusCondSet.Manage(s).MarkFalse(PrometheusConditionValidSchedule, reason, messageFormat, messageA...)
}

// MarkSinkWarnDeprecated sets the condition that the source has a sink configured and warns ref is deprecated.
func (s *PrometheusSourceStatus) MarkSinkWarnRefDeprecated(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		c := apis.Condition{
			Type:     PrometheusConditionSinkProvided,
			Status:   corev1.ConditionTrue,
			Severity: apis.ConditionSeverityError,
			Message:  "Using deprecated object ref fields when specifying spec.sink. These will be removed in a future release. Update to spec.sink.ref.",
		}
		PrometheusCondSet.Manage(s).SetCondition(c)
	} else {
		PrometheusCondSet.Manage(s).MarkUnknown(PrometheusConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkSink sets the condition that the source has a sink configured.
func (s *PrometheusSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		PrometheusCondSet.Manage(s).MarkTrue(PrometheusConditionSinkProvided)
	} else {
		PrometheusCondSet.Manage(s).MarkUnknown(PrometheusConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *PrometheusSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	PrometheusCondSet.Manage(s).MarkFalse(PrometheusConditionSinkProvided, reason, messageFormat, messageA...)
}

// PropagateDeploymentAvailability uses the availability of the provided Deployment to determine if
// PrometheusConditionDeployed should be marked as true or false.
func (s *PrometheusSourceStatus) PropagateDeploymentAvailability(d *appsv1.Deployment) {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		PrometheusCondSet.Manage(s).MarkTrue(PrometheusConditionDeployed)
	} else {
		// I don't know how to propagate the status well, so just give the name of the Deployment
		// for now.
		PrometheusCondSet.Manage(s).MarkFalse(PrometheusConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

// MarkEventTypes sets the condition that the source has set its event type.
func (s *PrometheusSourceStatus) MarkEventTypes() {
	PrometheusCondSet.Manage(s).MarkTrue(PrometheusConditionEventTypeProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event type configured.
func (s *PrometheusSourceStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	PrometheusCondSet.Manage(s).MarkFalse(PrometheusConditionEventTypeProvided, reason, messageFormat, messageA...)
}

// IsReady returns true if the resource is ready overall.
func (s *PrometheusSourceStatus) IsReady() bool {
	return PrometheusCondSet.Manage(s).IsHappy()
}
