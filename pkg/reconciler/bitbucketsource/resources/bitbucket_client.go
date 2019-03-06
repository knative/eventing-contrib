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

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultBaseUrl   = "https://api.bitbucket.org/2.0/"
	defaultUserAgent = "go-bitbucket"
	mediaTypeJson    = "application/json"
)

// Hook struct to marshal/unmarshal BitBucket requests/responses.
type Hook struct {
	URL         string   `json:"url,omitempty"`
	Description string   `json:"description,omitempty"`
	Events      []string `json:"events,omitempty"`
	Active      bool     `json:"active,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
}

// Client struct used to send http.Requests to BitBucket.
type Client struct {
	client    *http.Client
	logger    *zap.SugaredLogger
	token     *oauth2.Token
	baseUrl   *url.URL
	userAgent string
}

// Wrapper struct in case we want to add extra validations here.
type Response struct {
	*http.Response
}

// newResponse creates a new Response for the provided http.Response.
// r must not be nil.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	return response

}

// NewClient creates a new Client for sending http.Requests to BitBucket.
func NewClient(ctx context.Context, token *oauth2.Token) *Client {
	logger := logging.FromContext(ctx)
	httpClient := http.DefaultClient
	baseUrl, _ := url.Parse(defaultBaseUrl)
	return &Client{client: httpClient, logger: logger, token: token, baseUrl: baseUrl, userAgent: defaultUserAgent}
}

// CreateHook creates a WebHook for 'owner' and 'repo'.
func (c *Client) CreateHook(owner, repo string, hook *Hook) (*Hook, *Response, error) {
	body, err := createHookBody(hook)
	if err != nil {
		return nil, nil, err
	}
	var urlStr string
	if repo == "" {
		// For every repo of the owner.
		urlStr = fmt.Sprintf("teams/%s/hooks", owner)
	} else {
		// For a specific repo of the owner.
		urlStr = fmt.Sprintf("repositories/%s/%s/hooks", owner, repo)
	}

	c.logger.Infof("BitBucket CreateHook URL %s", urlStr)

	h := new(Hook)
	resp, err := c.doRequest("POST", urlStr, body, h)
	if err != nil {
		return nil, resp, err
	}
	return h, resp, nil
}

// DeleteHook deletes the WebHook 'hookUUID' previously registered for 'owner' and 'repo'.
func (c *Client) DeleteHook(owner, repo, hookUUID string) (*Response, error) {

	var urlStr string
	if repo == "" {
		urlStr = fmt.Sprintf("teams/%v/hooks/%s", owner, hookUUID)
	} else {
		urlStr = fmt.Sprintf("repositories/%v/%v/hooks/%s", owner, repo, hookUUID)
	}

	c.logger.Infof("BitBucket DeleteHook URL %s", urlStr)

	resp, err := c.doRequest("DELETE", urlStr, "", nil)

	if err != nil {
		return resp, err
	}
	return resp, nil
}

// createHookBody marshals the WebHook body into json format.
func createHookBody(hook *Hook) (string, error) {
	body := map[string]interface{}{}
	body["description"] = hook.Description
	body["url"] = hook.URL
	body["events"] = hook.Events
	body["active"] = hook.Active
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// doRequest performs an http.Request to BitBucket. If v is not nil, it attempts to unmarshal the response to
// that particular struct.
func (c *Client) doRequest(method, urlStr string, body string, v interface{}) (*Response, error) {
	u, err := c.baseUrl.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	c.logger.Infof("BitBucket Request URL %s", u.String())

	b := strings.NewReader(body)
	req, err := http.NewRequest(method, u.String(), b)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", mediaTypeJson)
	req.Header.Set("Accept", mediaTypeJson)
	// Add the Oauth2 accessToken in the Authorization header.
	req.Header.Set("Authorization", "Bearer "+c.token.AccessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	// Just checking for 200, 201, and 204 status codes as those are the success status codes for creating and deleting hooks.
	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) && (resp.StatusCode != http.StatusNoContent) {
		return nil, fmt.Errorf("invalid status %q: %v", resp.Status, resp)
	}

	response := newResponse(resp)

	if v != nil {
		if resp.Body != nil {
			decErr := json.NewDecoder(resp.Body).Decode(v)
			if decErr != nil {
				err = decErr
			}
			return response, err
		}
	}
	return response, nil
}
