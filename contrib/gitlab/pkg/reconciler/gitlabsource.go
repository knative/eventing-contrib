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

package gitlabsource

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/gitlab/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/contrib/gitlab/pkg/reconciler/resources"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/pkg/logging"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "gitlab-source-controller"
	raImageEnvVar       = "GL_RA_IMAGE"
	finalizerName       = controllerAgentName
)

// Add creates a new GitLabSource Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the
// Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager, logger *zap.SugaredLogger) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	log.Println("Adding the Gitlab Source controller.")
	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &sourcesv1alpha1.GitLabSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			recorder:            mgr.GetRecorder(controllerAgentName),
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: receiveAdapterImage,
			webhookClient:       gitLabWebhookClient{}, //TODO
		},
	}

	return p.Add(mgr, logger)
}

// reconciler reconciles a GitLabSource object
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	recorder            record.EventRecorder
	receiveAdapterImage string
	webhookClient       webhookClient
}

type webhookArgs struct {
	source      *sourcesv1alpha1.GitLabSource
	domain      string
	accessToken string
	secretToken string
	projectURL  string
	hookID      string
}

// Reconcile reads that state of the cluster for a GitLabSource
// object and makes changes based on the state read and what is in the
// GitLabSource.Spec
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	source, ok := object.(*sourcesv1alpha1.GitLabSource)
	if !ok {
		logger.Errorf("could not find gitlab source %v\n", object)
		return nil
	}

	// See if the source has been deleted
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", zap.Error(err))
		return err
	}

	var reconcileErr error
	if accessor.GetDeletionTimestamp() == nil {
		reconcileErr = r.reconcile(ctx, source)
	} else {
		reconcileErr = r.finalize(ctx, source)
	}

	return reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.GitLabSource) error {
	source.Status.InitializeConditions()

	accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
		return err
	}
	secretToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("SecretTokenNotFound", "%s", err)
		return err
	}
	source.Status.MarkSecrets()

	uri, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedService(ctx, source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc = resources.MakeService(source, r.receiveAdapterImage)
			if err = controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
				return err
			}
			if err = r.client.Create(ctx, ksvc); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created Service %q", ksvc.Name)
			// TODO: Mark Deploying for the ksvc
			// Wait for the Service to get a status
			return nil
		}
		// Error was something other than NotFound
		return err
	}

	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
	receiveAdapterDomain := ksvc.Status.Domain
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
		// TODO: Mark Deployed for the ksvc
		// TODO: Mark some condition for the webhook status?
		r.addFinalizer(source)
		if source.Status.WebhookIDKey == "" {
			args := &webhookArgs{
				source:      source,
				domain:      receiveAdapterDomain,
				accessToken: accessToken,
				secretToken: secretToken,
			}

			hookID, err := r.createWebhook(ctx, args)
			if err != nil {
				return err
			}
			source.Status.WebhookIDKey = hookID
		}
	}
	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.GitLabSource) error {
	// Always remove the finalizer. If there's a failure cleaning up, an event
	// will be recorded allowing the webhook to be removed manually by the
	// operator.
	r.removeFinalizer(source)

	// If a webhook was created, try to delete it
	if source.Status.WebhookIDKey != "" {
		// Get access token
		accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return err
		}

		args := &webhookArgs{
			source:      source,
			accessToken: accessToken,
			projectURL:  source.Spec.ProjectURL,
			hookID:      source.Status.WebhookIDKey,
		}
		// Delete the webhook using the access token and stored webhook ID
		err = r.deleteWebhook(ctx, args)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FailedFinalize", "Could not delete webhook %q: %v", source.Status.WebhookIDKey, err)
			return err
		}
		// Webhook deleted, clear ID
		source.Status.WebhookIDKey = ""
	}
	return nil
}

func getProjectName(projectURL string) (string, error) {
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	return projectName, nil
}

func (r *reconciler) createWebhook(ctx context.Context, args *webhookArgs) (string, error) {
	logger := logging.FromContext(ctx)

	logger.Info("creating GitLab webhook")

	hookOptions := &projectHookOptions{
		accessToken: args.accessToken,
		secretToken: args.secretToken,
	}

	projectName, err := getProjectName(args.source.Spec.ProjectURL)
	if err != nil {
		return "", fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}
	hookOptions.project = projectName
	hookOptions.webhookID = args.source.Status.WebhookIDKey

	for _, event := range args.source.Spec.EventTypes {
		switch event {
		case "push_events":
			hookOptions.PushEvents = true
		case "issues_events":
			hookOptions.IssuesEvents = true
		case "confidential_issues_events":
			hookOptions.ConfidentialIssuesEvents = true
		case "merge_requests_events":
			hookOptions.MergeRequestsEvents = true
		case "tag_push_events":
			hookOptions.TagPushEvents = true
		case "pipeline_events":
			hookOptions.PipelineEvents = true
		case "wiki_page_events":
			hookOptions.WikiPageEvents = true
		case "job_events":
			hookOptions.JobEvents = true
		case "note_events":
			hookOptions.NoteEvents = true
		}
	}

	if args.source.Spec.Secure {
		hookOptions.url = "https://" + args.domain
		hookOptions.EnableSSLVerification = true
	} else {
		hookOptions.url = "http://" + args.domain
	}
	baseURL, err := getGitlabBaseURL(args.source.Spec.ProjectURL)
	if err != nil {
		return "", fmt.Errorf("Failed to process project url to get the base url: " + err.Error())
	}

	hookID, err := r.webhookClient.Create(ctx, hookOptions, baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to create webhook: %v", err)
	}
	return hookID, nil
}

func getGitlabBaseURL(projectURL string) (string, error) {
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	baseurl := strings.TrimSuffix(projectURL, projectName)
	return baseurl, nil
}

func (r *reconciler) deleteWebhook(ctx context.Context, args *webhookArgs) error {
	logger := logging.FromContext(ctx)

	logger.Info("deleting GitLab webhook")

	hookOptions := &projectHookOptions{
		accessToken: args.accessToken,
	}
	projectName, err := getProjectName(args.projectURL)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}
	hookOptions.project = projectName
	hookOptions.webhookID = args.source.Status.WebhookIDKey
	baseURL, err := getGitlabBaseURL(args.projectURL)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the base url: " + err.Error())
	}

	err = r.webhookClient.Delete(ctx, hookOptions, baseURL)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %v", err)
	}
	return nil
}

func (r *reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)
	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func (r *reconciler) getOwnedService(ctx context.Context, source *sourcesv1alpha1.GitLabSource) (*servingv1alpha1.Service, error) {
	list := &servingv1alpha1.ServiceList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: servingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			//TODO if there are >1 controlled, delete all but first?
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

func (r *reconciler) addFinalizer(s *sourcesv1alpha1.GitLabSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(s *sourcesv1alpha1.GitLabSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
