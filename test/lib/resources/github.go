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

package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
)

// GitHubSourceOptionV1Alpha1 enables further configuration of a GitHubSource.
type GitHubSourceOptionV1Alpha1 func(source *v1alpha1.GitHubSource)

// NewGitHubSourceV1Alpha1 creates a GitHubSource with GithubSourceOption.
func NewGitHubSourceV1Alpha1(name, namespace string, o ...GitHubSourceOptionV1Alpha1) *v1alpha1.GitHubSource {
	c := &v1alpha1.GitHubSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range o {
		opt(c)
	}
	c.SetDefaults(context.Background()) // TODO: We should add defaults and validation.
	return c
}

func NewSecretValueFromSource(name, key string) v1alpha1.SecretValueFromSource {
	return v1alpha1.SecretValueFromSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		},
	}
}

func WithGitHubSecretTokenSpecV1Alpha1(name, key string) GitHubSourceOptionV1Alpha1 {
	return func(c *v1alpha1.GitHubSource) {
		c.Spec.SecretToken = NewSecretValueFromSource(name, key)
	}
}

func WithGitHubAccessTokenSpecV1Alpha1(name, key string) GitHubSourceOptionV1Alpha1 {
	return func(c *v1alpha1.GitHubSource) {
		c.Spec.AccessToken = NewSecretValueFromSource(name, key)
	}
}

func WithGitHubSourceSpecV1Alpha1(spec v1alpha1.GitHubSourceSpec) GitHubSourceOptionV1Alpha1 {
	return func(c *v1alpha1.GitHubSource) {
		c.Spec = spec
	}
}

func WithGithubSourceConditionsV1Alpha1(s *v1alpha1.GitHubSource) {
	s.Status.InitializeConditions()
}

func WithGithubSourceSecretV1Alpha1(s *v1alpha1.GitHubSource) {
	s.Status.MarkSecrets()
}

func WithGithubSourceSinkV1Alpha1(url *apis.URL) GitHubSourceOptionV1Alpha1 {
	return func(s *v1alpha1.GitHubSource) {
		s.Status.MarkSink(url)
	}
}

func WithGithubSourceWebHookConfiguredV1Alpha1(s *v1alpha1.GitHubSource) {
	s.Status.MarkWebhookConfigured()
}
