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
	"golang.org/x/oauth2/bitbucket"
	"golang.org/x/oauth2/clientcredentials"
)

type webhookOptions struct {
	uuid        string
	accessToken string
	secretToken string
	domain      string
	owner       string
	repo        string
	events      []string
}

type webhookClient interface {
	Create(ctx context.Context, options *webhookOptions) (string, error)
	Delete(ctx context.Context, options *webhookOptions) error
}

type bitBucketWebhookClient struct{}

func (client bitBucketWebhookClient) Create(ctx context.Context, options *webhookOptions) (string, error) {
	logger := logging.FromContext(ctx)

	bbClient, err := client.createBitBucketClient(ctx, options)

	if err != nil {
		logger.Errorf("create bitbucket client error: %v", err)
		return "", err
	}

	hook := client.hookConfig(options)

	var h *bbclient.Hook
	var resp *bbclient.Response

	h, resp, err = bbClient.CreateHook(ctx, options.owner, options.repo, &hook)

	if err != nil {
		logger.Errorf("create webhook error response: %v", resp)
		return "", fmt.Errorf("failed to create the webhook: %v", err)
	}
	logger.Infof("created hook: %+v", h)

	return h.UUID, nil
}

func (client bitBucketWebhookClient) Delete(ctx context.Context, options *webhookOptions) error {
	logger := logging.FromContext(ctx)

	bbClient, err := client.createBitBucketClient(ctx, options)

	if err != nil {
		logger.Errorf("create bitbucket client error: %v", err)
		return err
	}

	hook := client.hookConfig(options)

	var resp *bbclient.Response
	resp, err = bbClient.DeleteHook(options.uuid, options.owner, options.repo)

	if err != nil {
		logger.Errorf("delete webhook error response: %v", resp)
		return fmt.Errorf("failed to delete the webhook: %v", err)
	}

	logger.Infof("deleted hook: %s", hook.Description)
	return nil
}

func (client bitBucketWebhookClient) createBitBucketClient(ctx context.Context, options *webhookOptions) (*bbclient.Client, error) {
	logger := logging.FromContext(ctx)

	conf := &clientcredentials.Config{
		ClientID:     options.accessToken,
		ClientSecret: options.secretToken,
		TokenURL:     bitbucket.Endpoint.TokenURL,
	}

	token, err := conf.Token(ctx)
	if err != nil {
		logger.Errorf("Error retrieving token: %v", err)
		return nil, err
	}

	return bbclient.NewClient(token.AccessToken), nil
}

func (client bitBucketWebhookClient) hookConfig(options *webhookOptions) bbclient.Hook {
	hook := bbclient.Hook{
		Description: "knative-sources",
		URL:         fmt.Sprintf("http://%s", options.domain),
		Events:      options.events,
		Active:      true,
	}
	return hook
}
