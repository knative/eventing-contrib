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

package adapter

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	gh "gopkg.in/go-playground/webhooks.v5/github"
)

const (
	GHHeaderEvent    = "X-GitHub-Event"
	GHHeaderDelivery = "X-GitHub-Delivery"
)

var validEvents = []gh.Event{
	gh.CheckSuiteEvent,
	gh.CommitCommentEvent,
	gh.CommitCommentEvent,
	gh.CreateEvent,
	gh.DeleteEvent,
	gh.DeploymentEvent,
	gh.DeploymentStatusEvent,
	gh.ForkEvent,
	gh.GollumEvent,
	gh.InstallationEvent,
	gh.IntegrationInstallationEvent,
	gh.IssueCommentEvent,
	gh.IssuesEvent,
	gh.LabelEvent,
	gh.MemberEvent,
	gh.MembershipEvent,
	gh.MilestoneEvent,
	gh.OrganizationEvent,
	gh.OrgBlockEvent,
	gh.PageBuildEvent,
	gh.PingEvent,
	gh.ProjectCardEvent,
	gh.ProjectColumnEvent,
	gh.ProjectEvent,
	gh.PublicEvent,
	gh.PullRequestEvent,
	gh.PullRequestReviewEvent,
	gh.PullRequestReviewCommentEvent,
	gh.PushEvent,
	gh.ReleaseEvent,
	gh.RepositoryEvent,
	gh.StatusEvent,
	gh.TeamEvent,
	gh.TeamAddEvent,
	gh.WatchEvent,
}

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing GitHub secret token
	EnvSecret string `envconfig:"GITHUB_SECRET_TOKEN" required:"true"`
	// Environment variable containing the HTTP port
	EnvPort string `envconfig:"PORT" default:"8080"`
	// Environment variable containing information about the origin of the event
	EnvOwnerRepo string `envconfig:"GITHUB_OWNER_REPO" required:"true"`
}

// NewEnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// gitHubAdapter converts incoming GitHub webhook events to CloudEvents
type gitHubAdapter struct {
	logger      *zap.SugaredLogger
	client      cloudevents.Client
	secretToken string
	port        string
	source      string
}

// NewAdapter returns the instance of gitHubReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	return &gitHubAdapter{
		logger:      logger,
		client:      ceClient,
		port:        env.EnvPort,
		secretToken: env.EnvSecret,
		source:      sourcesv1alpha1.GitHubEventSource(env.EnvOwnerRepo),
	}
}

// Start implements adapter.Adapter
func (a *gitHubAdapter) Start(ctx context.Context) error {
	return a.start(ctx.Done())
}

func (a *gitHubAdapter) start(stopCh <-chan struct{}) error {
	src := cloudevents.ParseURIRef(a.source)
	if src == nil {
		return fmt.Errorf("invalid source for github events: %s", a.source)
	}
	done := make(chan bool, 1)
	hook, err := gh.New(gh.Options.Secret(a.secretToken))
	if err != nil {
		return fmt.Errorf("cannot create gitHub hook: %v", err)
	}

	server := &http.Server{
		Addr:    ":" + a.port,
		Handler: a.newRouter(hook),
	}

	go gracefulShutdown(server, a.logger, stopCh, done)

	a.logger.Infof("Server is ready to handle requests at %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", server.Addr, err)
	}

	<-done
	a.logger.Infof("Server stopped")
	return nil
}

func (a *gitHubAdapter) newRouter(hook *gh.Webhook) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r, validEvents...)
		if err != nil {
			if err == gh.ErrEventNotFound {
				w.WriteHeader(http.StatusNotFound)
				a.logger.Info("Event not found")
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			a.logger.Errorf("Error processing request: %v", err)
			return
		}
		err = a.HandleEvent(payload, r.Header)
		if err != nil {
			a.logger.Errorf("Event handler error: %v", err)
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}
		a.logger.Infof("Event processed")
		w.WriteHeader(202)
		w.Write([]byte("accepted"))
	})
	return router
}

func gracefulShutdown(server *http.Server, logger *zap.SugaredLogger, stopCh <-chan struct{}, done chan<- bool) {
	<-stopCh
	logger.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v", err)
	}
	close(done)
}

// HandleEvent is invoked whenever an event comes in from GitHub
func (a *gitHubAdapter) HandleEvent(payload interface{}, header http.Header) error {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	return err
}

func (a *gitHubAdapter) handleEvent(payload interface{}, hdr http.Header) error {
	gitHubEventType := hdr.Get(GHHeaderEvent)
	if gitHubEventType == "" {
		return fmt.Errorf("%q header is not set", GHHeaderEvent)
	}
	eventID := hdr.Get(GHHeaderDelivery)
	if eventID == "" {
		return fmt.Errorf("%q header is not set", GHHeaderDelivery)
	}

	a.logger.Infof("Handling %s", gitHubEventType)

	cloudEventType := sourcesv1alpha1.GitHubEventType(gitHubEventType)
	subject := subjectFromGitHubEvent(gh.Event(gitHubEventType), payload, a.logger)

	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetType(cloudEventType)
	event.SetSource(a.source)
	event.SetSubject(subject)
	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	result := a.client.Send(context.Background(), event)
	if !cloudevents.IsACK(result) {
		return result
	}
	return nil
}

func subjectFromGitHubEvent(gitHubEvent gh.Event, payload interface{}, logger *zap.SugaredLogger) string {
	// The decision of what to put in subject is somewhat arbitrary here (i.e., it's the author's opinion)
	// TODO check if we should be setting subject to these values.
	var subject string
	var ok bool
	switch gitHubEvent {
	case gh.CheckSuiteEvent:
		var cs gh.CheckSuitePayload
		if cs, ok = payload.(gh.CheckSuitePayload); ok {
			subject = strconv.FormatInt(cs.CheckSuite.ID, 10)
		}
	case gh.CommitCommentEvent:
		var cc gh.CommitCommentPayload
		if cc, ok = payload.(gh.CommitCommentPayload); ok {
			// E.g., https://github.com/Codertocat/Hello-World/commit/a10867b14bb761a232cd80139fbd4c0d33264240#commitcomment-29186860
			// and we keep with a10867b14bb761a232cd80139fbd4c0d33264240#commitcomment-29186860
			subject = lastPathPortion(cc.Comment.HTMLURL)
		}
	case gh.CreateEvent:
		var c gh.CreatePayload
		if c, ok = payload.(gh.CreatePayload); ok {
			// The object that was created, can be repository, branch, or tag.
			subject = c.RefType
		}
	case gh.DeleteEvent:
		var d gh.DeletePayload
		if d, ok = payload.(gh.DeletePayload); ok {
			// The object that was deleted, can be branch or tag.
			subject = d.RefType
		}
	case gh.DeploymentEvent:
		var d gh.DeploymentPayload
		if d, ok = payload.(gh.DeploymentPayload); ok {
			subject = strconv.FormatInt(d.Deployment.ID, 10)
		}
	case gh.DeploymentStatusEvent:
		var d gh.DeploymentStatusPayload
		if d, ok = payload.(gh.DeploymentStatusPayload); ok {
			subject = strconv.FormatInt(d.Deployment.ID, 10)
		}
	case gh.ForkEvent:
		var f gh.ForkPayload
		if f, ok = payload.(gh.ForkPayload); ok {
			subject = strconv.FormatInt(f.Forkee.ID, 10)
		}
	case gh.GollumEvent:
		var g gh.GollumPayload
		if g, ok = payload.(gh.GollumPayload); ok {
			// The pages that were updated.
			// E.g., Home, Main.
			pages := make([]string, 0, len(g.Pages))
			for _, page := range g.Pages {
				pages = append(pages, page.PageName)
			}
			subject = strings.Join(pages, ",")
		}
	case gh.InstallationEvent, gh.IntegrationInstallationEvent:
		var i gh.InstallationPayload
		if i, ok = payload.(gh.InstallationPayload); ok {
			subject = strconv.FormatInt(i.Installation.ID, 10)
		}
	case gh.IssueCommentEvent:
		var i gh.IssueCommentPayload
		if i, ok = payload.(gh.IssueCommentPayload); ok {
			// E.g., https://github.com/Codertocat/Hello-World/issues/2#issuecomment-393304133
			// and we keep with 2#issuecomment-393304133
			subject = lastPathPortion(i.Comment.HTMLURL)
		}
	case gh.IssuesEvent:
		var i gh.IssuesPayload
		if i, ok = payload.(gh.IssuesPayload); ok {
			subject = strconv.FormatInt(i.Issue.Number, 10)
		}
	case gh.LabelEvent:
		var l gh.LabelPayload
		if l, ok = payload.(gh.LabelPayload); ok {
			// E.g., :bug: Bugfix
			subject = l.Label.Name
		}
	case gh.MemberEvent:
		var m gh.MemberPayload
		if m, ok = payload.(gh.MemberPayload); ok {
			subject = strconv.FormatInt(m.Member.ID, 10)
		}
	case gh.MembershipEvent:
		var m gh.MembershipPayload
		if m, ok = payload.(gh.MembershipPayload); ok {
			subject = strconv.FormatInt(m.Member.ID, 10)
		}
	case gh.MilestoneEvent:
		var m gh.MilestonePayload
		if m, ok = payload.(gh.MilestonePayload); ok {
			subject = strconv.FormatInt(m.Milestone.Number, 10)
		}
	case gh.OrganizationEvent:
		var o gh.OrganizationPayload
		if o, ok = payload.(gh.OrganizationPayload); ok {
			// The action that was performed, can be member_added, member_removed, or member_invited.
			subject = o.Action
		}
	case gh.OrgBlockEvent:
		var o gh.OrgBlockPayload
		if o, ok = payload.(gh.OrgBlockPayload); ok {
			// The action performed, can be blocked or unblocked.
			subject = o.Action
		}
	case gh.PageBuildEvent:
		var p gh.PageBuildPayload
		if p, ok = payload.(gh.PageBuildPayload); ok {
			subject = strconv.FormatInt(p.ID, 10)
		}
	case gh.PingEvent:
		var p gh.PingPayload
		if p, ok = payload.(gh.PingPayload); ok {
			subject = strconv.Itoa(p.HookID)
		}
	case gh.ProjectCardEvent:
		var p gh.ProjectCardPayload
		if p, ok = payload.(gh.ProjectCardPayload); ok {
			// The action performed on the project card, can be created, edited, moved, converted, or deleted.
			subject = p.Action
		}
	case gh.ProjectColumnEvent:
		var p gh.ProjectColumnPayload
		if p, ok = payload.(gh.ProjectColumnPayload); ok {
			// The action performed on the project column, can be created, edited, moved, converted, or deleted.
			subject = p.Action
		}
	case gh.ProjectEvent:
		var p gh.ProjectPayload
		if p, ok = payload.(gh.ProjectPayload); ok {
			// The action that was performed on the project, can be created, edited, closed, reopened, or deleted.
			subject = p.Action
		}
	case gh.PublicEvent:
		var p gh.PublicPayload
		if p, ok = payload.(gh.PublicPayload); ok {
			subject = strconv.FormatInt(p.Repository.ID, 10)
		}
	case gh.PullRequestEvent:
		var p gh.PullRequestPayload
		if p, ok = payload.(gh.PullRequestPayload); ok {
			subject = strconv.FormatInt(p.PullRequest.Number, 10)
		}
	case gh.PullRequestReviewEvent:
		var p gh.PullRequestReviewPayload
		if p, ok = payload.(gh.PullRequestReviewPayload); ok {
			subject = strconv.FormatInt(p.Review.ID, 10)
		}
	case gh.PullRequestReviewCommentEvent:
		var p gh.PullRequestReviewCommentPayload
		if p, ok = payload.(gh.PullRequestReviewCommentPayload); ok {
			subject = strconv.FormatInt(p.Comment.ID, 10)
		}
	case gh.PushEvent:
		var p gh.PushPayload
		if p, ok = payload.(gh.PushPayload); ok {
			// E.g., https://github.com/Codertocat/Hello-World/compare/a10867b14bb7...000000000000
			// and we keep with a10867b14bb7...000000000000.
			subject = lastPathPortion(p.Compare)
		}
	case gh.ReleaseEvent:
		var r gh.ReleasePayload
		if r, ok = payload.(gh.ReleasePayload); ok {
			subject = r.Release.TagName
		}
	case gh.RepositoryEvent:
		var r gh.RepositoryPayload
		if r, ok = payload.(gh.RepositoryPayload); ok {
			subject = strconv.FormatInt(r.Repository.ID, 10)
		}
	case gh.StatusEvent:
		var s gh.StatusPayload
		if s, ok = payload.(gh.StatusPayload); ok {
			subject = s.Sha
		}
	case gh.TeamEvent:
		var t gh.TeamPayload
		if t, ok = payload.(gh.TeamPayload); ok {
			subject = strconv.FormatInt(t.Team.ID, 10)
		}
	case gh.TeamAddEvent:
		var t gh.TeamAddPayload
		if t, ok = payload.(gh.TeamAddPayload); ok {
			subject = strconv.FormatInt(t.Repository.ID, 10)
		}
	case gh.WatchEvent:
		var w gh.WatchPayload
		if w, ok = payload.(gh.WatchPayload); ok {
			subject = strconv.FormatInt(w.Repository.ID, 10)
		}
	}
	if !ok {
		logger.Errorf("Invalid payload in gitHub event %s", gitHubEvent)
	} else if subject == "" {
		logger.Warnf("No subject found in gitHub event %s", gitHubEvent)
	}
	return subject
}

func lastPathPortion(url string) string {
	var subject string
	index := strings.LastIndex(url, "/")
	if index != -1 {
		// Keep the last part.
		subject = url[index+1:]
	}
	return subject
}
