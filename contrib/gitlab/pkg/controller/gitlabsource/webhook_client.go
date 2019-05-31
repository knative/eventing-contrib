/*
Copyright 2019 The TriggerMesh Authors.

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

package gitlabsource

import (
	"fmt"
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

func (client gitlabHookClient) Create(baseUrl string, options *projectHookOptions) (string, error) {

	glClient := gitlab.NewClient(nil, options.accessToken)
	glClient.SetBaseURL(baseUrl)

	if options.id != "" {
		hookId, err := strconv.Atoi(options.id)
		if err != nil {
			return "", fmt.Errorf("failed to convert hook id to int: " + err.Error())
		}
		projhooks, _, err := glClient.Projects.ListProjectHooks(options.project, nil, nil)
		if err != nil {
			return "", fmt.Errorf("Failed to list project hooks for project:" + options.project + " due to" + err.Error())
		} else {
			for _, hook := range projhooks {
				if hook.ID == hookId {
					return options.id, nil
				}
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
		return "", fmt.Errorf("Failed to add webhook to the project:" + options.project + " due to " + err.Error())
	}

	return strconv.Itoa(hook.ID), nil
}

func (client gitlabHookClient) Delete(baseUrl string, options *projectHookOptions) error {
	if options.id != "" {
		hookId, err := strconv.Atoi(options.id)
		if err != nil {
			return fmt.Errorf("failed to convert hook id to int: " + err.Error())
		}
		glClient := gitlab.NewClient(nil, options.accessToken)
		glClient.SetBaseURL(baseUrl)

		projhooks, _, err := glClient.Projects.ListProjectHooks(options.project, nil, nil)
		if err != nil {
			return fmt.Errorf("Failed to list project hooks for project: " + options.project)
		} else {
			for _, hook := range projhooks {
				if hook.ID == hookId {
					_, err = glClient.Projects.DeleteProjectHook(options.project, hookId, nil)
					if err != nil {
						return fmt.Errorf("Failed to delete project hook: " + err.Error())
					}
				}
			}
		}
	}

	return nil
}
