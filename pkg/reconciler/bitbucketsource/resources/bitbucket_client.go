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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	defaultBaseUrl   = "https://api.bitbucket.org/2.0/"
	defaultUserAgent = "go-bitbucket"
	mediaTypeJson    = "application/json"
)

type Hook struct {
	Name        string   `json:"name,omitempty"`
	URL         string   `json:"url,omitempty"`
	Description string   `json:"description,omitempty"`
	Events      []string `json:"events,omitempty"`
	Active      bool     `json:"active,omitempty"`
	UUID        string   `json:"uuid,omitempty"`
	Secret      string   `json:"secret,omitempty"`
}

type Client struct {
	client    *http.Client
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

func NewClient(client *http.Client) *Client {
	baseUrl, _ := url.Parse(defaultBaseUrl)
	return &Client{client: client, baseUrl: baseUrl, userAgent: defaultUserAgent}
}

func (c *Client) CreateHook(owner, repo string, hook *Hook) (*Hook, *Response, error) {
	body, err := createHookBody(hook)
	if err != nil {
		return nil, nil, err
	}

	urlStr := fmt.Sprintf("repositories/%v/%v/hooks", owner, repo)

	h := new(Hook)
	resp, err := c.doRequest("POST", urlStr, body, h)
	if err != nil {
		return nil, resp, err
	}
	return h, resp, nil
}

func (c *Client) DeleteHook(hookUUID, owner, repo string) (*Response, error) {

	urlStr := fmt.Sprintf("repositories/%v/%v/hooks", owner, repo)

	resp, err := c.doRequest("DELETE", urlStr, nil, nil)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func createHookBody(hook *Hook) (string, error) {
	body := map[string]interface{}{}
	body["name"] = hook.Name
	body["description"] = hook.Description
	body["url"] = hook.URL
	body["active"] = true
	body["events"] = hook.Events
	body["secret"] = hook.Secret
	data, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *Client) doRequest(method, urlStr string, body interface{}, v interface{}) (*Response, error) {
	u, err := c.baseUrl.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err := enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, u.String(), buf)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", mediaTypeJson)
	req.Header.Set("Accept", mediaTypeJson)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
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
