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
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing-contrib/github"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"
)

func TestGitHubBindingGetGroupVersionKind(t *testing.T) {
	r := &GitHubBinding{}
	want := schema.GroupVersionKind{
		Group:   "bindings.knative.dev",
		Version: "v1alpha1",
		Kind:    "GitHubBinding",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestGitHubBindingGetters(t *testing.T) {
	r := &GitHubBinding{
		Spec: GitHubBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "foo",
				},
			},
		},
	}
	if got, want := r.GetUntypedSpec(), r.Spec; !reflect.DeepEqual(got, want) {
		t.Errorf("GetUntypedSpec() = %v, want: %v", got, want)
	}
	if got, want := r.GetSubject(), r.Spec.Subject; !reflect.DeepEqual(got, want) {
		t.Errorf("GetSubject() = %v, want: %v", got, want)
	}
	if got, want := r.GetBindingStatus(), &r.Status; !reflect.DeepEqual(got, want) {
		t.Errorf("GetBindingStatus() = %v, want: %v", got, want)
	}
}

func TestGitHubBindingSetObsGen(t *testing.T) {
	r := &GitHubBinding{
		Spec: GitHubBindingSpec{
			BindingSpec: duckv1alpha1.BindingSpec{
				Subject: tracker.Reference{
					APIVersion: "foo",
				},
			},
		},
	}
	want := int64(3762)
	r.GetBindingStatus().SetObservedGeneration(want)
	if got := r.Status.ObservedGeneration; got != want {
		t.Errorf("SetObservedGeneration() = %d, wanted %d", got, want)
	}
}

func TestGitHubBindingStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *GitHubBindingStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &GitHubBindingStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *GitHubBindingStatus {
			s := &GitHubBindingStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark available",
		s: func() *GitHubBindingStatus {
			s := &GitHubBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingUnavailable("TheReason", "this is the message")
			return s
		}(),
		want: false,
	}, {
		name: "mark available",
		s: func() *GitHubBindingStatus {
			s := &GitHubBindingStatus{}
			s.InitializeConditions()
			s.MarkBindingAvailable()
			return s
		}(),
		want: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestGitHubBindingUndo(t *testing.T) {
	secretName, secretKey := "name", "key"

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to remove",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
	}, {
		name: "lots to remove",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			sb := &GitHubBinding{
				Spec: GitHubBindingSpec{
					AccessToken: SecretValueFromSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: secretKey,
						},
					},
				},
			}
			sb.Undo(context.Background(), got)

			if !cmp.Equal(got, test.want) {
				t.Errorf("Undo (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestGitHubBindingDo(t *testing.T) {
	secretName, secretKey := "name", "key"

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to add",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: github.VolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{{
										Key:  secretKey,
										Path: github.AccessTokenKey,
									}},
								},
							},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: github.VolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{{
										Key:  secretKey,
										Path: github.AccessTokenKey,
									}},
								},
							},
						}},
					},
				},
			},
		},
	}, {
		name: "fix the key",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: github.VolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{{
										Key:  "wrong-key",
										Path: github.AccessTokenKey,
									}},
								},
							},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: github.VolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{{
										Key:  secretKey,
										Path: github.AccessTokenKey,
									}},
								},
							},
						}},
					},
				},
			},
		},
	}, {
		name: "lots to add",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{{
							Name:  "setup",
							Image: "busybox",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "FOO",
								Value: "BAR",
							}, {
								Name:  "BAZ",
								Value: "INGA",
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}, {
							Name:  "sidecar",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "BAZ",
								Value: "INGA",
							}},
							VolumeMounts: []corev1.VolumeMount{{
								Name:      github.VolumeName,
								ReadOnly:  true,
								MountPath: github.MountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name: github.VolumeName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{{
										Key:  secretKey,
										Path: github.AccessTokenKey,
									}},
								},
							},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in

			sb := &GitHubBinding{Spec: GitHubBindingSpec{
				AccessToken: SecretValueFromSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: secretName,
						},
						Key: secretKey,
					},
				},
			}}
			sb.Do(context.Background(), got)

			if !cmp.Equal(got, test.want) {
				t.Errorf("Undo (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}
