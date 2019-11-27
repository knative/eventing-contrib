package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	"knative.dev/eventing/pkg/apis/duck"
	"knative.dev/pkg/apis"
)

const (
	RabbitmqConditionReady = apis.ConditionReady

	RabbitmqConditionSinkProvided apis.ConditionType = "SinkProvided"

	RabbitmqConditionDeployed apis.ConditionType = "Deployed"

	RabbitmqConditionResources apis.ConditionType = "ResourcesCorrect"
)

var RabbitmqSourceCondSet = apis.NewLivingConditionSet(
	RabbitmqConditionSinkProvided,
	RabbitmqConditionDeployed)

func (s *RabbitmqSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return RabbitmqSourceCondSet.Manage(s).GetCondition(t)
}

func (s *RabbitmqSourceStatus) IsReady() bool {
	return RabbitmqSourceCondSet.Manage(s).IsHappy()
}

func (s *RabbitmqSourceStatus) InitializeConditions()  {
	RabbitmqSourceCondSet.Manage(s).InitializeConditions()
}

func (s *RabbitmqSourceStatus) MarkSink(uri *apis.URL)  {
	s.SinkURI = uri
	if !uri.IsEmpty() {
		RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionSinkProvided)
	} else {
		RabbitmqSourceCondSet.Manage(s).MarkUnknown(RabbitmqConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

func (s *RabbitmqSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{})  {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionSinkProvided, reason, messageFormat, messageA...)
}

func DeploymentIsAvailable(d *appsv1.DeploymentStatus, def bool) bool {
	for _, cond := range d.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			return cond.Status == "True"
		}
	}
	return def
}

func (s *RabbitmqSourceStatus) MarkDeployed(d *appsv1.Deployment)  {
	if duck.DeploymentIsAvailable(&d.Status, false) {
		RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionDeployed)
	} else {
		RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionDeployed, "DeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
	}
}

func (s *RabbitmqSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{})  {
	RabbitmqSourceCondSet.Manage(s).MarkUnknown(RabbitmqConditionDeployed, reason, messageFormat, messageA...)
}

func (s *RabbitmqSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{})  {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionDeployed, reason, messageFormat, messageA...)
}

func (s *RabbitmqSourceStatus) MarkResourcesCorrect() {
	RabbitmqSourceCondSet.Manage(s).MarkTrue(RabbitmqConditionResources)
}

func (s *RabbitmqSourceStatus) MarkResourcesIncorrect(reason, messageFormat string, messageA ...interface{}) {
	RabbitmqSourceCondSet.Manage(s).MarkFalse(RabbitmqConditionResources, reason, messageFormat, messageA...)
}