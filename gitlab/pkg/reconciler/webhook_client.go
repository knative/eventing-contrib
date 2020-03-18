/*
Copyright 2020 The Knative Authors.

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

package gitlab

import (
	"fmt"
	"net/http"
	"strconv"

	gitlab "github.com/xanzy/go-gitlab"
)

type projectHookOptions struct {
	accessToken              string
	secretToken              string
	project                  string
	id                       string
	url                      string
	PushEvents               bool
	IssuesEvents             bool
	ConfidentialIssuesEvents bool
	MergeRequestsEvents      bool
	TagPushEvents            bool
	NoteEvents               bool
	JobEvents                bool
	PipelineEvents           bool
	WikiPageEvents           bool
	EnableSSLVerification    bool
}

type projectHookClient interface {
	Create(options *projectHookOptions) (string, error)
	Delete(options *projectHookOptions, hookID string) error
}

type gitlabHookClient struct{}

func (client gitlabHookClient) Create(baseURL string, options *projectHookOptions) (string, error) {
	glClient := gitlab.NewClient(nil, options.accessToken)
	glClient.SetBaseURL(baseURL)

	if options.id != "" {
		hookID, err := strconv.Atoi(options.id)
		if err != nil {
			return "", fmt.Errorf("failed to convert hook id to int: %s", err.Error())
		}
		projhooks, resp, err := glClient.Projects.ListProjectHooks(options.project,
			&gitlab.ListProjectHooksOptions{
				// Max number of hook per project
				PerPage: 100,
			}, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list project hooks for project %q due to an error: %s", options.project, err.Error())
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("project hooks list unexpected status: %s", resp.Status)
		}
		for _, hook := range projhooks {
			if hook.ID == hookID {
				return options.id, nil
			}
		}
	}

	hookOptions := gitlab.AddProjectHookOptions{
		URL:                      &options.url,
		PushEvents:               &options.PushEvents,
		IssuesEvents:             &options.IssuesEvents,
		ConfidentialIssuesEvents: &options.ConfidentialIssuesEvents,
		MergeRequestsEvents:      &options.MergeRequestsEvents,
		TagPushEvents:            &options.TagPushEvents,
		NoteEvents:               &options.NoteEvents,
		JobEvents:                &options.JobEvents,
		PipelineEvents:           &options.PipelineEvents,
		WikiPageEvents:           &options.WikiPageEvents,
		Token:                    &options.secretToken,
		EnableSSLVerification:    &options.EnableSSLVerification,
	}

	hook, _, err := glClient.Projects.AddProjectHook(options.project, &hookOptions, nil)
	if err != nil {
		return "", fmt.Errorf("failed to add webhook to the project %q due to an error: %s ", options.project, err.Error())
	}

	return strconv.Itoa(hook.ID), nil
}

func (client gitlabHookClient) Delete(baseURL string, options *projectHookOptions) error {
	if options.id != "" {
		hookID, err := strconv.Atoi(options.id)
		if err != nil {
			return fmt.Errorf("failed to convert hook id to int: " + err.Error())
		}
		glClient := gitlab.NewClient(nil, options.accessToken)
		glClient.SetBaseURL(baseURL)

		projhooks, _, err := glClient.Projects.ListProjectHooks(options.project, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list project hooks for project: " + options.project)
		}
		for _, hook := range projhooks {
			if hook.ID == hookID {
				_, err = glClient.Projects.DeleteProjectHook(options.project, hookID, nil)
				if err != nil {
					return fmt.Errorf("Failed to delete project hook: " + err.Error())
				}
			}
		}
	}

	return nil
}
