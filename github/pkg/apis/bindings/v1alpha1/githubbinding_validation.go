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

	"knative.dev/pkg/apis"
)

// Validate implements apis.Validatable
func (fb *GitHubBinding) Validate(ctx context.Context) *apis.FieldError {
	err := fb.Spec.Validate(ctx).ViaField("spec")
	if fb.Spec.Subject.Namespace != "" && fb.Namespace != fb.Spec.Subject.Namespace {
		err = err.Also(apis.ErrInvalidValue(fb.Spec.Subject.Namespace, "spec.subject.namespace"))
	}
	return err
}

// Validate implements apis.Validatable
func (fbs *GitHubBindingSpec) Validate(ctx context.Context) *apis.FieldError {
	err := fbs.Subject.Validate(ctx).ViaField("subject")
	if fbs.AccessToken.SecretKeyRef == nil {
		err = err.Also(apis.ErrMissingField("accessToken.secretKeyRef"))
	} else {
		if fbs.AccessToken.SecretKeyRef.Name == "" {
			err = err.Also(apis.ErrMissingField("accessToken.secretKeyRef.name"))
		}
		if fbs.AccessToken.SecretKeyRef.Key == "" {
			err = err.Also(apis.ErrMissingField("accessToken.secretKeyRef.key"))
		}
	}
	return err
}
