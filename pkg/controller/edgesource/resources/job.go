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
	"encoding/json"
	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"

	"fmt"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Operation specifies the operation for the feed container to perform.
type Operation string

const (
	// OperationStartJob specifies a job should be started
	OperationStartJob Operation = "START"
	// OperationStopJob specifies a job should be stopped
	OperationStopJob = "STOP"
)

// EnvVar specifies the names of the environment variables passed to the
// feed container.
type EnvVar string

const (
	// EnvVarOperation is the Env variable that gets set to requested Operation
	EnvVarOperation EnvVar = "SRC_OPERATION"
	// EnvVarTrigger is the Env variable that gets set to serialized trigger configuration
	EnvVarTrigger = "FEED_TRIGGER"
	// EnvVarTarget is the Env variable that gets set to target of the feed operation
	EnvVarSink = "SRC_SINK_URI"
	// EnvVarContext is the Env variable that gets set to serialized FeedContext if stopping
	EnvVarContext = "FEED_CONTEXT"
	// EnvVarEventSourceParameters is the Env variable that gets set to serialized EventSourceSpec
	EnvVarEventSourceParameters = "EVENT_SOURCE_PARAMETERS"
	// EnvVarNamespace is the Env variable that gets set to namespace of the container doing
	// the Feed (aka, namespace of the feed). Uses downward api
	EnvVarNamespace = "FEED_NAMESPACE"
	// EnvVarServiceAccount is the Env variable that gets set to serviceaccount of the
	// container doing the feed. Uses downward api
	//TODO is this useful? Wouldn't this already be the implicit service Account
	// for the container?
	EnvVarServiceAccount = "FEED_SERVICE_ACCOUNT"
)

var (
	// DefaultBackoffLimit is the default BackoffLimit value for feedlet jobs.
	// No more than this number of retry pods will be created before the job is
	// considered failed. The total number of tries is this number + 1.
	DefaultBackoffLimit int32 = 2
	// DefaultActiveDeadlineSeconds is the default ActiveDeadlineSeconds value for
	// feedlet jobs. The job cannot be active for more than this number of
	// seconds before it is considered failed.
	DefaultActiveDeadlineSeconds int64 = 30
)

// MakeJob creates a Job to start or stop a Feed.
func MakeJob(source *v1alpha1.EdgeSource, job *v1alpha1.JobSpec) (*batchv1.Job, error) {
	labels := map[string]string{
		"app": "pod",
	}

	podTemplate, err := makePodTemplate(source, job)
	if err != nil {
		return nil, err
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    JobName(source, job),
			Namespace:       source.Namespace,
			Labels:          labels,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(source, v1alpha1.SchemeGroupVersion.WithKind("EdgeSource"))},
		},
		Spec: batchv1.JobSpec{
			Template:              *podTemplate,
			BackoffLimit:          &DefaultBackoffLimit,
			ActiveDeadlineSeconds: &DefaultActiveDeadlineSeconds,
		},
	}, nil
}

// IsJobComplete returns true if the Job has completed successfully, or false if
// the Job is in progress or failed.
func IsJobComplete(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// IsJobFailed returns true if the Job has failed, or false if
// the Job is in progress or completed successfully.
func IsJobFailed(job *batchv1.Job) bool {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// JobFailedMessage returns a string containing the job's failure reason
// and message for use in a Condition.
func JobFailedMessage(job *batchv1.Job) string {
	for _, c := range job.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return fmt.Sprintf("[%s] %s", c.Reason, c.Message)
		}
	}
	return ""
}

// makePodTemplate creates a pod template for a feed stop or start Job.
// TOO: update text
func makePodTemplate(source *v1alpha1.EdgeSource, job *v1alpha1.JobSpec) (*corev1.PodTemplateSpec, error) {
	var op Operation

	switch job.Type {
	case v1alpha1.EdgeJobStart:
		op = OperationStartJob
	case v1alpha1.EdgeJobStop:
		op = OperationStopJob
	}

	var marshaledContext []byte
	var err error
	// Check for an existing RawExtension object in the Feed status
	if rawExt := source.Status.JobContext; rawExt != nil && rawExt.Raw != nil && len(rawExt.Raw) > 0 {
		var ctx JobContext
		if err = json.Unmarshal(rawExt.Raw, &ctx.Context); err != nil {
			return nil, err
		}
		marshaledContext, err = json.Marshal(ctx)
		if err != nil {
			return nil, err
		}
	}
	// If no context was present, marshal an empty context because the event
	// source wrapper expects it to be valid json.
	if len(marshaledContext) == 0 {
		marshaledContext, err = json.Marshal(JobContext{})
		if err != nil {
			return nil, err
		}
	}
	//encodedContext := base64.StdEncoding.EncodeToString(marshaledContext)

	//marshalledTrigger, err := json.Marshal(trigger)
	//if err != nil {
	//	return nil, err
	//}
	//encodedTrigger := base64.StdEncoding.EncodeToString(marshalledTrigger)
	//
	//var encodedSourceParameters string
	//if source.Spec.Parameters != nil {
	//	marshalledSourceParameters, err := json.Marshal(source.Spec.Parameters)
	//	if err != nil {
	//		return nil, err
	//	}
	//	encodedSourceParameters = base64.StdEncoding.EncodeToString(marshalledSourceParameters)
	//}

	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			ServiceAccountName: source.Spec.ServiceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "source-job",
					Image:           source.Spec.Image,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env: []corev1.EnvVar{
						{
							Name:  string(EnvVarOperation),
							Value: string(op),
						},
						{
							Name:  string(EnvVarSink),
							Value: source.Status.SinkURI,
						},
						//{
						//	Name:  string(EnvVarTrigger),
						//	Value: encodedTrigger,
						//},
						//{
						//	Name:  string(EnvVarContext),
						//	Value: encodedContext,
						//},
						//{
						//	Name:  string(EnvVarEventSourceParameters),
						//	Value: encodedSourceParameters,
						//},
						{
							Name: string(EnvVarNamespace),
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name: string(EnvVarServiceAccount),
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.serviceAccountName",
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// GetFirstTerminationMessage returns the termination message of the first
// terminated container in the given Pod.
func GetFirstTerminationMessage(pod *corev1.Pod) string {
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Terminated != nil && cs.State.Terminated.Message != "" {
			return cs.State.Terminated.Message
		}
	}
	return ""
}
