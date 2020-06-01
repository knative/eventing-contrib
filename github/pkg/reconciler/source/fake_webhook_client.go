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

package source

import (
	"context"
)

func NewFakeWebhookClient() webhookClient {
	return &fakeGitHubWebhookClient{}
}

type fakeGitHubWebhookClient struct{}

func (client fakeGitHubWebhookClient) Create(ctx context.Context, options *webhookOptions, alternateGitHubAPIURL string) (string, error) {
	return "fake-id", nil
}

func (client fakeGitHubWebhookClient) Reconcile(ctx context.Context, options *webhookOptions, hookID, alternateGitHubAPIURL string) error {
	return nil
}

func (client fakeGitHubWebhookClient) Delete(ctx context.Context, options *webhookOptions, hookID, alternateGitHubAPIURL string) error {
	return nil
}
