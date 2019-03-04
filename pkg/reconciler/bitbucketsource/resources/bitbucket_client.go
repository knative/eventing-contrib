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
	"net/http"
	"net/url"
	"strings"
)

const (
	defaultBaseUrl   = "https://api.bitbucket.org/2.0/"
	defaultUserAgent = "go-bitbucket"
	mediaTypeJson    = "application/json"
)

type Hook struct {
	URL         string   `json:"url,omitempty"`
	Description string   `json:"description,omitempty"`
	Events      []string `json:"events,omitempty"`
	Active      bool     `json:"active,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
}

type Client struct {
	client    *http.Client
	accessToken string
	baseUrl   *url.URL
	userAgent string
}

type Response struct {
	*http.Response
}

// newResponse creates a new Response for the provided http.Response.
// r must not be nil.
func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	return response
}

func NewClient(accessToken string) *Client {
	httpClient := http.DefaultClient
	baseUrl, _ := url.Parse(defaultBaseUrl)
	return &Client{client: httpClient, accessToken: accessToken, baseUrl: baseUrl, userAgent: defaultUserAgent}
}

func (c *Client) CreateHook(ctx context.Context, owner, repo string, hook *Hook) (*Hook, *Response, error) {
	logger := logging.FromContext(ctx)

	body, err := createHookBody(hook)
	if err != nil {
		return nil, nil, err
	}
	logger.Infof("Request body %s", body)


	urlStr := fmt.Sprintf("repositories/%v/%v/hooks", owner, repo)

	logger.Infof("Request URL %s", urlStr)

	h := new(Hook)
	resp, err := c.doRequest(ctx,"POST", urlStr, body, h)
	if err != nil {
		return nil, resp, err
	}
	return h, resp, nil
}

func (c *Client) DeleteHook(hookUUID, owner, repo string) (*Response, error) {

	urlStr := fmt.Sprintf("repositories/%v/%v/hooks/%s", owner, repo, hookUUID)

	resp, err := c.doRequest(nil, "DELETE", urlStr, "", nil)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

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

func (c *Client) doRequest(ctx context.Context, method, urlStr string, body string, v interface{}) (*Response, error) {
	logger := logging.FromContext(ctx)
	u, err := c.baseUrl.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	logger.Infof("Request URL %s", u.String())

	b := strings.NewReader(body)
	req, err := http.NewRequest(method, u.String(), b)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", mediaTypeJson)
	req.Header.Set("Accept", mediaTypeJson)
	req.Header.Set("Authorization", "Bearer " + c.accessToken)

	resp, err := c.client.Do(req)
	if err != nil {
		logger.Errorf("Error doing request %v", err)
		return nil, err
	}

	if resp.Body != nil {
		defer resp.Body.Close()
	}

	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
		logger.Errorf("Response %v", resp)
		return nil, fmt.Errorf(resp.Status)
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
