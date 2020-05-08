/*
Copyright 2020 The Knative Authors

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
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing-contrib/gitlab"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracker"
)

var sbCondSet = apis.NewLivingConditionSet()

// GetGroupVersionKind returns the GroupVersionKind.
func (s *GitLabBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GitLabBinding")
}

// GetUntypedSpec implements apis.HasSpec
func (s *GitLabBinding) GetUntypedSpec() interface{} {
	return s.Spec
}

// GetSubject implements psbinding.Bindable
func (sb *GitLabBinding) GetSubject() tracker.Reference {
	return sb.Spec.Subject
}

// GetBindingStatus implements psbinding.Bindable
func (sb *GitLabBinding) GetBindingStatus() duck.BindableStatus {
	return &sb.Status
}

// SetObservedGeneration implements psbinding.BindableStatus
func (sbs *GitLabBindingStatus) SetObservedGeneration(gen int64) {
	sbs.ObservedGeneration = gen
}

// InitializeConditions populates the GitLabBindingStatus's conditions field
// with all of its conditions configured to Unknown.
func (sbs *GitLabBindingStatus) InitializeConditions() {
	sbCondSet.Manage(sbs).InitializeConditions()
}

// MarkBindingUnavailable marks the GitLabBinding's Ready condition to False with
// the provided reason and message.
func (sbs *GitLabBindingStatus) MarkBindingUnavailable(reason, message string) {
	sbCondSet.Manage(sbs).MarkFalse(GitLabBindingConditionReady, reason, message)
}

// MarkBindingAvailable marks the GitLabBinding's Ready condition to True.
func (sbs *GitLabBindingStatus) MarkBindingAvailable() {
	sbCondSet.Manage(sbs).MarkTrue(GitLabBindingConditionReady)
}

// Do implements psbinding.Bindable
func (sb *GitLabBinding) Do(ctx context.Context, ps *duckv1.WithPod) {
	// First undo so that we can just unconditionally append below.
	sb.Undo(ctx, ps)

	// Make sure the PodSpec has a Volume like this:
	volume := corev1.Volume{
		Name: gitlab.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: sb.Spec.AccessToken.SecretKeyRef.Name,
				Items: []corev1.KeyToPath{{
					Key:  sb.Spec.AccessToken.SecretKeyRef.Key,
					Path: gitlab.AccessTokenKey,
				}},
			},
		},
	}
	ps.Spec.Template.Spec.Volumes = append(ps.Spec.Template.Spec.Volumes, volume)

	// Make sure that each [init]container in the PodSpec has a VolumeMount like this:
	volumeMount := corev1.VolumeMount{
		Name:      gitlab.VolumeName,
		ReadOnly:  true,
		MountPath: gitlab.MountPath,
	}
	spec := ps.Spec.Template.Spec
	for i := range spec.InitContainers {
		spec.InitContainers[i].VolumeMounts = append(spec.InitContainers[i].VolumeMounts, volumeMount)
	}
	for i := range spec.Containers {
		spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, volumeMount)
	}
}

func (sb *GitLabBinding) Undo(ctx context.Context, ps *duckv1.WithPod) {
	spec := ps.Spec.Template.Spec

	// Make sure the PodSpec does NOT have the gitlab volume.
	for i, v := range spec.Volumes {
		if v.Name == gitlab.VolumeName {
			ps.Spec.Template.Spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	// Make sure that none of the [init]containers have the gitlab volume mount
	for i, c := range spec.InitContainers {
		for j, vm := range c.VolumeMounts {
			if vm.Name == gitlab.VolumeName {
				spec.InitContainers[i].VolumeMounts = append(c.VolumeMounts[:j], c.VolumeMounts[j+1:]...)
				break
			}
		}
	}

	for i, c := range spec.Containers {
		for j, vm := range c.VolumeMounts {
			if vm.Name == gitlab.VolumeName {
				spec.Containers[i].VolumeMounts = append(c.VolumeMounts[:j], c.VolumeMounts[j+1:]...)
				break
			}
		}
	}
}
