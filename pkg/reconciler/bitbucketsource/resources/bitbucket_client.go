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
	"net/http"
)

type Hook struct {
	UUID        string
	Name        string
	URL         string
	Description string
	Events      []string
	Secret      string
}

type Client struct {
	client *http.Client
}

type Response struct {
}

func NewClient(client *http.Client) *Client {
	return &Client{client: client}

}

func (client *Client) CreateHook(ctx context.Context, owner, repo string, hook *Hook) (*Hook, *Response, error) {
	return nil, nil, nil
}

func (client *Client) DeleteHook(ctx context.Context, hookUUID, owner, repo string) (*Response, error) {
	return nil, nil
}
