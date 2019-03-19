/*
Copyright 2019 The Knative Authors

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

package reconciler

import (
	"context"
	"fmt"

	"github.com/knative/eventing-sources/contrib/bitbucket/pkg/bbclient"
	"github.com/knative/pkg/logging"
	"golang.org/x/oauth2/bitbucket"
	"golang.org/x/oauth2/clientcredentials"
)

type webhookOptions struct {
	uuid           string
	consumerKey    string
	consumerSecret string
	domain         string
	owner          string
	repo           string
	events         []string
}

type webhookClient interface {
	Create(ctx context.Context, options *webhookOptions) (string, error)
	Delete(ctx context.Context, options *webhookOptions) error
}

type bitBucketWebhookClient struct{}

func (client bitBucketWebhookClient) Create(ctx context.Context, options *webhookOptions) (string, error) {
	logger := logging.FromContext(ctx)

	logger.Info("Creating BitBucket WebHook")

	bbClient, err := createBitBucketClient(ctx, options)

	if err != nil {
		return "", err
	}

	hook := hookConfig(options)

	var h *bbclient.Hook
	h, err = bbClient.CreateHook(options.owner, options.repo, &hook)

	if err != nil {
		return "", fmt.Errorf("failed to Create the BitBucket Webhook: %v", err)
	}
	logger.Infof("Created BitBucket WebHook: %+v", h)
	return h.UUID, nil
}

func (client bitBucketWebhookClient) Delete(ctx context.Context, options *webhookOptions) error {
	logger := logging.FromContext(ctx)

	logger.Info("Deleting BitBucket WebHook: %q", options.uuid)

	bbClient, err := createBitBucketClient(ctx, options)

	if err != nil {
		return err
	}

	err = bbClient.DeleteHook(options.owner, options.repo, options.uuid)

	if err != nil {
		return fmt.Errorf("failed to Delete the BitBucket Webhook: %v", err)
	}

	logger.Infof("Deleted BitBucket Webhook: %s", options.uuid)
	return nil
}

func createBitBucketClient(ctx context.Context, options *webhookOptions) (*bbclient.Client, error) {
	// Retrieve an access token based on the Oauth Consumer credentials we set in the BitBucket account.
	// It cannot be a static one as it can expire.
	conf := &clientcredentials.Config{
		ClientID:     options.consumerKey,
		ClientSecret: options.consumerSecret,
		TokenURL:     bitbucket.Endpoint.TokenURL,
	}
	token, err := conf.Token(ctx)
	if err != nil {
		return nil, err
	}

	return bbclient.NewClient(ctx, token), nil
}

func hookConfig(options *webhookOptions) bbclient.Hook {
	hook := bbclient.Hook{
		Description: "knative-sources",
		URL:         fmt.Sprintf("http://%s", options.domain),
		Events:      options.events,
		Active:      true,
	}
	return hook
}
