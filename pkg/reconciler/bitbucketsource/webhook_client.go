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

package bitbucketsouce

import (
	"context"
	"fmt"
	bbclient "github.com/knative/eventing-sources/pkg/reconciler/bitbucketsource/resources"
	"github.com/knative/pkg/logging"
	"golang.org/x/oauth2"
)

type webhookOptions struct {
	accessToken string
	secretToken string
	domain      string
	owner       string
	repo        string
	events      []string
}

type webhookClient interface {
	Create(ctx context.Context, options *webhookOptions) (string, error)
	Delete(ctx context.Context, hookUUID, options *webhookOptions) error
}

type bitBucketWebhookClient struct{}

func (client bitBucketWebhookClient) Create(ctx context.Context, options *webhookOptions) (string, error) {
	var err error
	logger := logging.FromContext(ctx)

	bbClient := client.createBitBucketClient(ctx, options)

	hook := client.hookConfig(ctx, options)

	var h *bbclient.Hook
	var resp *bbclient.Response

	h, resp, err = bbClient.CreateHook(ctx, options.owner, options.repo, &hook)

	if err != nil {
		logger.Infof("create webhook error response:\n%+v", resp)
		return "", fmt.Errorf("failed to create the webhook: %v", err)
	}
	logger.Infof("created hook: %+v", h)

	return h.UUID, nil
}

func (client bitBucketWebhookClient) Delete(ctx context.Context, hookUUID string, options *webhookOptions) error {
	var err error
	logger := logging.FromContext(ctx)

	bbClient := client.createBitBucketClient(ctx, options)

	hook := client.hookConfig(ctx, options)

	var resp *bbclient.Response
	resp, err = bbClient.DeleteHook(ctx, hookUUID, options.owner, options.repo)

	if err != nil {
		logger.Infof("delete webhook error response:\n%+v", resp)
		return fmt.Errorf("failed to delete the webhook: %v", err)
	}

	logger.Infof("deleted hook: %s", hook.Name)
	return nil
}

func (client bitBucketWebhookClient) createBitBucketClient(ctx context.Context, options *webhookOptions) *bbclient.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: options.accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	return bbclient.NewClient(tc)
}

func (client bitBucketWebhookClient) hookConfig(ctx context.Context, options *webhookOptions) bbclient.Hook {
	hookname := "knative-sources"
	hook := bbclient.Hook{
		Name:   hookname,
		URL:    fmt.Sprintf("http://%s", options.domain),
		Events: options.events,
		Secret: options.secretToken,
	}
	return hook
}
