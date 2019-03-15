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

package githubsource

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	ghclient "github.com/google/go-github/github"
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
	secure      bool
}

type webhookClient interface {
	Create(ctx context.Context, options *webhookOptions, alternateGitHubAPIURL string) (string, error)
	Delete(ctx context.Context, options *webhookOptions, hookID, alternateGitHubAPIURL string) error
}

type gitHubWebhookClient struct{}

func (client gitHubWebhookClient) Create(ctx context.Context, options *webhookOptions, alternateGitHubAPIURL string) (string, error) {
	var err error
	logger := logging.FromContext(ctx)

	ghClient := client.createGitHubClient(ctx, options)
	if alternateGitHubAPIURL != "" {
		//This is to support GitHub Enterprise, this might be something like https://github.company.com/api/v3/
		ghClient.BaseURL, err = url.Parse(alternateGitHubAPIURL)
		if err != nil {
			logger.Infof("Failed to create webhook because an error occurred parsing githubAPIURL %q, error was: %v", alternateGitHubAPIURL, err)
			return "", fmt.Errorf("error occurred parsing githubAPIURL: %v", err)
		}
	}

	hook := client.hookConfig(ctx, options)

	var h *ghclient.Hook
	var resp *ghclient.Response
	if options.repo != "" {
		h, resp, err = ghClient.Repositories.CreateHook(ctx, options.owner, options.repo, &hook)
	} else {
		h, resp, err = ghClient.Organizations.CreateHook(ctx, options.owner, &hook)
	}
	if err != nil {
		logger.Infof("create webhook error response:\n%+v", resp)
		return "", fmt.Errorf("failed to create the webhook: %v", err)
	}
	logger.Infof("created hook: %+v", h)

	return strconv.FormatInt(*h.ID, 10), nil
}

func (client gitHubWebhookClient) Delete(ctx context.Context, options *webhookOptions, hookID, alternateGitHubAPIURL string) error {
	logger := logging.FromContext(ctx)

	hookIDInt, err := strconv.ParseInt(hookID, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert webhook %q to int64: %v", hookID, err)
	}

	ghClient := client.createGitHubClient(ctx, options)
	if alternateGitHubAPIURL != "" {
		ghClient.BaseURL, err = url.Parse(alternateGitHubAPIURL)
		if err != nil {
			logger.Infof("Failed to delete webhook because an error occurred parsing githubAPIURL %q, error was: %v", alternateGitHubAPIURL, err)
			return fmt.Errorf("error occurred parsing githubAPIURL: %v", err)
		}
	}

	hook := client.hookConfig(ctx, options)

	var resp *ghclient.Response
	if options.repo != "" {
		resp, err = ghClient.Repositories.DeleteHook(ctx, options.owner, options.repo, hookIDInt)
	} else {
		resp, err = ghClient.Organizations.DeleteHook(ctx, options.owner, hookIDInt)
	}
	if err != nil {
		logger.Infof("delete webhook error response:\n%+v", resp)
		return fmt.Errorf("failed to delete the webhook: %v", err)
	}

	logger.Infof("deleted hook: %s", hook.Name)
	return nil
}

func (client gitHubWebhookClient) createGitHubClient(ctx context.Context, options *webhookOptions) *ghclient.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: options.accessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	return ghclient.NewClient(tc)
}

func (client gitHubWebhookClient) hookConfig(ctx context.Context, options *webhookOptions) ghclient.Hook {
	domain := options.domain
	active := true
	config := make(map[string]interface{})
	protocol := "http"
	if options.secure {
		protocol = "https"
	}
	config["url"] = fmt.Sprintf("%s://%s", protocol, domain)
	config["content_type"] = "json"
	config["secret"] = options.secretToken

	// GitHub hook names are required to be named "web" or the name of a GitHub service
	hookname := "web"
	hook := ghclient.Hook{
		Name:   &hookname,
		URL:    &domain,
		Events: options.events,
		Active: &active,
		Config: config,
	}
	return hook
}
