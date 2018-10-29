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

package edgesource

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/edgesource/resources"
	"github.com/knative/pkg/logging"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcileStartJob creates a start Job if one doesn't exist, checks the status
// of the start Job, and updates the Feed status accordingly.
// TODO: update text
func (r *reconciler) reconcileStartJob(ctx context.Context, source *v1alpha1.EdgeSource, jobSpec *v1alpha1.JobSpec) error {
	jc := source.Status.GetCondition(v1alpha1.EdgeConditionJob)
	switch jc.Status {
	case corev1.ConditionUnknown:

		jobContext, err := r.getJobContext(ctx, source)
		if err != nil {
			return err
		}

		job := &batchv1.Job{}
		jobName := jobContext.StartJobName

		if err := r.client.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(ctx, source, jobSpec)
				if err != nil {
					return err
				}
				r.recorder.Eventf(source, corev1.EventTypeNormal, "StartJobCreated", "Created start job %q", job.Name)
				source.Status.MarkJobStarting("StartJob", "start job in progress")
				jobContext.StartJobName = job.Name
			}
		}
		r.addFinalizer(source)

		if resources.IsJobComplete(job) {
			r.recorder.Eventf(source, corev1.EventTypeNormal, "StartJobCompleted", "Start job %q completed", job.Name)
			if err := r.setJobContext(ctx, source, job); err != nil {
				return err
			}
			source.Status.MarkJobStarted()
		} else if resources.IsJobFailed(job) {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "StartJobFailed", "Start job %q failed: %q", job.Name, resources.JobFailedMessage(job))
			source.Status.MarkJobFailed("JobFailed", "Job failed with %s", resources.JobFailedMessage(job))
		}

	case corev1.ConditionTrue:
		//TODO delete job
	}
	return nil
}

// reconcileStopJob deletes the start Job if it exists, creates a stop Job if
// one doesn't exist, checks the status of the stop Job, and updates the Feed
// status accordingly.
// TODO: update text
func (r *reconciler) reconcileStopJob(ctx context.Context, source *v1alpha1.EdgeSource, jobSpec *v1alpha1.JobSpec) error {
	logger := logging.FromContext(ctx)
	if r.hasFinalizer(source) {

		jobContext, err := r.getJobContext(ctx, source)
		if err != nil {
			return err
		}

		// check for an existing start Job
		job := &batchv1.Job{}
		jobName := jobContext.StartJobName

		err = r.client.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: jobName}, job)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err == nil {
			// Delete the existing job and return. When it's deleted, this Feed
			// will be reconciled again.
			logger.Infof("Found existing start job: %s/%s", job.Namespace, job.Name)

			// Need to delete pods first to workaround the client's lack of support
			// for cascading deletes. TODO remove this when client support allows.
			if err = r.deleteJobPods(ctx, job); err != nil {
				return err
			}
			r.client.Delete(ctx, job)
			logger.Infof("Deleted start job: %s/%s", job.Namespace, job.Name)
			return nil
		}

		jobName = jobContext.StopJobName // TODO: this will be empty normally but the next bit of code queries for the empty name

		if err := r.client.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: jobName}, job); err != nil {
			if errors.IsNotFound(err) {
				job, err = r.createJob(ctx, source, jobSpec)
				if err != nil {
					return err
				}
				r.recorder.Eventf(source, corev1.EventTypeNormal, "StopJobCreated", "Created stop job %q", job.Name)
				//TODO check for event source not found and remove finalizer

				source.Status.MarkJobStarting("StopJob", "stop job in progress")
				jobContext.StopJobName = job.Name
				r.setJobContext(ct)
			}
		}

		if resources.IsJobComplete(job) {
			r.recorder.Eventf(source, corev1.EventTypeNormal, "StopJobCompleted", "Stop job %q completed", job.Name)
			r.removeFinalizer(source)
			source.Status.MarkJobStarted() // TODO: mark that the stop job succeeded
		} else if resources.IsJobFailed(job) {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "StopJobFailed", "Stop job %q failed: %q", job.Name, resources.JobFailedMessage(job))
			logger.Warnf("Stop job %q failed, removing finalizer on source %q anyway.", job.Name, source.Name)
			r.removeFinalizer(source)
			source.Status.MarkJobFailed("JobFailed", "Job failed with %s", resources.JobFailedMessage(job))
		}
	}
	return nil
}

// createJob creates a Job for the given Feed based on its current state,
// returning the created Job.
func (r *reconciler) createJob(ctx context.Context, source *v1alpha1.EdgeSource, jobSpec *v1alpha1.JobSpec) (*batchv1.Job, error) {
	if source.Status.SinkURI == "" {
		return nil, fmt.Errorf("sink not resolved")
	}
	job, err := resources.MakeJob(source, jobSpec)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, job); err != nil {
		return nil, err
	}
	return job, nil
}

// setFeedContext sets the Feed's context from the context emitted by the given
// Job.
// TODO: update text.
func (r *reconciler) setJobContext(ctx context.Context, source *v1alpha1.EdgeSource, job *batchv1.Job) error {
	jobContext, err := r.makeJobContext(ctx, job)
	if err != nil {
		return err
	}

	marshalledFeedContext, err := json.Marshal(&jobContext.Context) // TODO this blows away old context
	if err != nil {
		return err
	}
	source.Status.JobContext = &runtime.RawExtension{
		Raw: marshalledFeedContext,
	}

	return nil
}

func (r *reconciler) updateJobContext(ctx context.Context, source *v1alpha1.EdgeSource, jobContext *resources.JobContext) error {
	marshalledFeedContext, err := json.Marshal(&jobContext.Context) // TODO this blows away old context
	if err != nil {
		return err
	}
	source.Status.JobContext = &runtime.RawExtension{
		Raw: marshalledFeedContext,
	}
	return nil
}

// makeJobContext returns the FeedContext emitted by the first successful pod
// owned by this job. The feed context is extracted from the termination
// message of the first container in the pod.
// TODO: update text.
func (r *reconciler) makeJobContext(ctx context.Context, job *batchv1.Job) (*resources.JobContext, error) {
	logger := logging.FromContext(ctx)
	pods, err := r.getJobPods(ctx, job)
	if err != nil {
		return nil, err
	}

	for _, p := range pods {
		if p.Status.Phase == corev1.PodSucceeded {
			logger.Infof("Pod succeeded: %s", p.Name)
			if msg := resources.GetFirstTerminationMessage(&p); msg != "" {
				decodedContext, _ := base64.StdEncoding.DecodeString(msg)
				logger.Infof("decoded to %q", decodedContext)
				var ret resources.JobContext
				err = json.Unmarshal(decodedContext, &ret)
				if err != nil {
					logger.Errorf("failed to unmarshal context: %s", err)
					return nil, err
				}
				return &ret, nil
			}
		}
	}
	return &resources.JobContext{}, nil
}

// makeJobContext returns the FeedContext emitted by the first successful pod
// owned by this job. The feed context is extracted from the termination
// message of the first container in the pod.
// TODO: update text.
func (r *reconciler) getJobContext(ctx context.Context, source *v1alpha1.EdgeSource) (*resources.JobContext, error) {
	logger := logging.FromContext(ctx)

	if source.Status.JobContext != nil {
		var ret resources.JobContext
		err := json.Unmarshal(source.Status.JobContext.Raw, &ret)
		if err != nil {
			logger.Errorf("failed to unmarshal context: %s", err)
			return nil, err
		}
		return &ret, nil
	}
	return &resources.JobContext{}, nil
}

// getJobPods returns the array of Pods owned by the given Job.
func (r *reconciler) getJobPods(ctx context.Context, job *batchv1.Job) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	listOptions := client.
		InNamespace(job.Namespace).
		MatchingLabels(job.Spec.Selector.MatchLabels)

	//TODO this is here because the fake client needs it. Remove this when it's
	// no longer needed.
	listOptions.Raw = &metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Pod",
		},
	}

	if err := r.client.List(ctx, listOptions, podList); err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (r *reconciler) deleteJobPods(ctx context.Context, job *batchv1.Job) error {
	logger := logging.FromContext(ctx)
	pods, err := r.getJobPods(ctx, job)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if err := r.client.Delete(ctx, &pod); err != nil {
			return err
		}
		logger.Infof("Deleted start job pod: %s/%s", pod.Namespace, pod.Name)
	}
	return nil
}
