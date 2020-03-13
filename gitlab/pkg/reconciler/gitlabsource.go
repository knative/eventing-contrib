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
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	//k8s.io imports

	"github.com/juju/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/kmeta"

	//knative.dev/serving imports

	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"
	servinglisters "knative.dev/serving/pkg/client/listers/serving/v1"

	//knative.dev/eventing-contrib imports
	sourcesv1alpha1 "knative.dev/eventing-contrib/gitlab/pkg/apis/sources/v1alpha1"
	clientset "knative.dev/eventing-contrib/gitlab/pkg/client/clientset/versioned"
	listers "knative.dev/eventing-contrib/gitlab/pkg/client/listers/sources/v1alpha1"

	//knative.dev/eventing imports
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"

	//knative.dev/pkg imports

	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

const (
	controllerAgentName = "gitlab-source-controller"
	finalizerName       = controllerAgentName
	raImageEnvVar       = "GL_RA_IMAGE"

	gitLabSourceUpdateStatusFailed = "GitLabSourceUpdateStatusFailed"
	gitLabSourceReadinessChanged   = "GitLabSourceReadinessChanged"
	gitLabSourceReconciled         = "GitLabSourceReconciled"
)

var receiveAdapterImage string

// Reconciler reconciles a GitLabSource object
type Reconciler struct {
	*reconciler.Base

	gitlabClientSet clientset.Interface
	gitlabLister    listers.GitLabSourceLister

	servingClientSet servingclientset.Interface
	servingLister    servinglisters.ServiceLister

	receiveAdapterImage string
	eventTypeLister     eventinglisters.EventTypeLister

	deploymentLister appsv1listers.DeploymentLister
	sinkResolver     *resolver.URIResolver

	loggingContext context.Context
	loggingConfig  *logging.Config
}

// Reconcile reads that state of the cluster for a GitLabSource object and makes changes based on the state read
// and what is in the GitLabSource.Spec
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Error("invalid resource key: %s", zap.Any("key", key))
	}

	logger.Info("Reconciling " + namespace)

	// Fetch the GitLabSource instance
	source, err := r.gitlabLister.GitLabSources(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// Finalizers ensure that controller gets chance to process gitlabsource  delete
			return nil
		}
		// Error reading the object - requeue the request.
		return err
	}

	// Don't modify the original (informers) copy
	gitlab := source.DeepCopy()

	var reconcileErr error
	if gitlab.ObjectMeta.GetDeletionTimestamp() == nil {
		reconcileErr = r.reconcile(gitlab)
	} else {
		if r.hasFinalizer(gitlab) {
			reconcileErr = r.finalize(gitlab)
		}
		return reconcileErr
	}
	if _, updateStatusErr := r.updateStatus(ctx, gitlab.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the GitLabSource", zap.Error(updateStatusErr))
		r.Recorder.Eventf(gitlab, corev1.EventTypeWarning, gitLabSourceUpdateStatusFailed, "Failed to update GitLabSource's status: %v", updateStatusErr)
		return updateStatusErr
	}

	return reconcileErr
}

func getGitlabBaseURL(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	baseURL := strings.TrimSuffix(projectUrl, projectName)
	return baseURL, nil
}

func getProjectName(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	return projectName, nil
}

func (r *Reconciler) reconcile(source *sourcesv1alpha1.GitLabSource) error {
	source.Status.InitializeConditions()

	projectName, err := getProjectName(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}

	hookOptions := projectHookOptions{}
	hookOptions.project = projectName
	hookOptions.id = source.Status.Id

	for _, event := range source.Spec.EventTypes {
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
	hookOptions.accessToken, err = r.secretFrom(source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecret("NotFound", "%s", err)
		return err
	}
	hookOptions.secretToken, err = r.secretFrom(source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecret("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSecret()

	sink := source.Spec.Sink.DeepCopy()

	if sink.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if sink.Ref.Namespace == "" {
			//TODO how does this work with deprecated fields
			sink.Ref.Namespace = source.GetNamespace()
		}
	}

	uri, err := r.sinkResolver.URIFromDestinationV1(*sink, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedKnativeService(source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc = r.generateKnativeServiceObject(source, r.receiveAdapterImage)
			// if err = controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
			// return fmt.Errorf("Failed to create knative service for the gitlabsource: " + err.Error())
			// }
			if _, err = r.servingClientSet.ServingV1().Services(ksvc.GetNamespace()).Create(ksvc); err != nil {
				return nil
			}
		} else {
			return fmt.Errorf("Failed to verify if knative service is created for the gitlabsource: " + err.Error())
		}
	}

	ksvc, err = r.waitForKnativeServiceReady(source)
	if err != nil {
		return err
	}
	hookOptions.url = ksvc.Status.URL.String()
	if source.Spec.SslVerify {
		hookOptions.EnableSSLVerification = true
	}

	baseURL, err := getGitlabBaseURL(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the base url: " + err.Error())
	}
	gitlabClient := gitlabHookClient{}
	hookID, err := gitlabClient.Create(baseURL, &hookOptions)
	if err != nil {
		return fmt.Errorf("Failed to create project hook: " + err.Error())
	}
	source.Status.Id = hookID
	r.addFinalizer(source)
	return nil
}

func (r *Reconciler) finalize(source *sourcesv1alpha1.GitLabSource) error {
	if source.Status.Id != "" {
		hookOptions := projectHookOptions{}
		projectName, err := getProjectName(source.Spec.ProjectUrl)
		if err != nil {
			return fmt.Errorf("failed to process project url to get the project name: %s", err.Error())
		}
		hookOptions.project = projectName
		hookOptions.id = source.Status.Id
		hookOptions.accessToken, err = r.secretFrom(source.Namespace, source.Spec.AccessToken.SecretKeyRef)
		if err != nil {
			return err
		}
		baseURL, err := getGitlabBaseURL(source.Spec.ProjectUrl)
		if err != nil {
			return fmt.Errorf("failed to process project url to get the base url: %s", err.Error())
		}
		gitlabClient := gitlabHookClient{}
		err = gitlabClient.Delete(baseURL, &hookOptions)
		if err != nil {
			return fmt.Errorf("failed to delete project hook: %s", err.Error())
		}
		source.Status.Id = ""
	}
	r.removeFinalizer(source)
	_, err := r.gitlabClientSet.SourcesV1alpha1().GitLabSources(source.Namespace).Update(source)
	return err
}

func (r *Reconciler) secretFrom(namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	secret, err := r.KubeClientSet.CoreV1().Secrets(namespace).Get(secretKeySelector.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func (r *Reconciler) addFinalizer(src *sourcesv1alpha1.GitLabSource) error {
	finalizers := sets.NewString(src.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(src.Finalizers, finalizerName),
			"resourceVersion": src.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return nil
	}
	_, err = r.gitlabClientSet.SourcesV1alpha1().GitLabSources(src.Namespace).Patch(src.Name, types.MergePatchType, patch)
	return err
}

func (r *Reconciler) removeFinalizer(source *sourcesv1alpha1.GitLabSource) {
	finalizers := sets.NewString(source.Finalizers...)
	finalizers.Delete(finalizerName)
	source.Finalizers = finalizers.List()
}

func (r *Reconciler) hasFinalizer(source *sourcesv1alpha1.GitLabSource) bool {
	for _, finalizerStr := range source.Finalizers {
		if finalizerStr == finalizerName {
			return true
		}
	}
	return false
}

func (r *Reconciler) generateKnativeServiceObject(source *sourcesv1alpha1.GitLabSource, receiveAdapterImage string) *servingv1.Service {
	labels := map[string]string{
		"receive-adapter": "gitlab",
	}
	env := []corev1.EnvVar{
		{
			Name: "GITLAB_SECRET_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: source.Spec.SecretToken.SecretKeyRef,
			},
		}, {
			Name:  "K_SINK",
			Value: source.Status.SinkURI,
		}, {
			Name:  "NAMESPACE",
			Value: source.GetNamespace(),
		}, {
			Name:  "METRICS_DOMAIN",
			Value: "knative.dev/eventing",
		}, {
			Name:  "K_METRICS_CONFIG",
			Value: "",
		}, {
			Name:  "K_LOGGING_CONFIG",
			Value: "",
		},
	}
	return &servingv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", source.Name),
			Namespace:    source.Namespace,
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(source),
			},
		},
		Spec: servingv1.ServiceSpec{
			ConfigurationSpec: servingv1.ConfigurationSpec{
				Template: servingv1.RevisionTemplateSpec{
					Spec: servingv1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							ServiceAccountName: source.Spec.ServiceAccountName,
							Containers: []corev1.Container{
								{
									Image: receiveAdapterImage,
									Env:   env,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *Reconciler) getOwnedKnativeService(source *sourcesv1alpha1.GitLabSource) (*servingv1.Service, error) {
	list, err := r.servingClientSet.ServingV1().Services(source.GetNamespace()).List(metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
	})

	if err != nil {
		return nil, err
	}
	for _, ksvc := range list.Items {
		if metav1.IsControlledBy(&ksvc, source) {
			return &ksvc, nil
		}
	}

	return nil, apierrors.NewNotFound(servingv1.Resource("services"), "")
}

// UpdateFromLoggingConfigMap loads logger configuration from configmap
func (r *Reconciler) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")
	}

	logcfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		logging.FromContext(r.loggingContext).Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
		return
	}
	r.loggingConfig = logcfg
	logging.FromContext(r.loggingContext).Info("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
}

func (r *Reconciler) waitForKnativeServiceReady(source *sourcesv1alpha1.GitLabSource) (*servingv1.Service, error) {
	for attempts := 0; attempts < 4; attempts++ {
		ksvc, err := r.getOwnedKnativeService(source)
		if err != nil {
			return nil, err
		}
		routeCondition := ksvc.Status.GetCondition(servingv1.ServiceConditionRoutesReady)
		receiveAdapterDomain := ksvc.Status.URL.String()
		if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
			return ksvc, nil
		}
		time.Sleep(2000 * time.Millisecond)
	}
	return nil, fmt.Errorf("Failed to get service to be in ready state")
}

func (r *Reconciler) updateStatus(ctx context.Context, source *sourcesv1alpha1.GitLabSource) (*sourcesv1alpha1.GitLabSource, error) {
	gitlab, err := r.gitlabLister.GitLabSources(source.Namespace).Get(source.Name)
	if err != nil {
		return nil, err
	}

	// If the status remains the same (nothing to update), return.
	if reflect.DeepEqual(gitlab.Status, source.Status) {
		return gitlab, nil
	}

	becomesReady := source.Status.IsReady() && !gitlab.Status.IsReady()

	// We avoid modifying the informer's copy
	existing := gitlab.DeepCopy()
	existing.Status = source.Status

	result, err := r.gitlabClientSet.SourcesV1alpha1().GitLabSources(source.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(result.ObjectMeta.CreationTimestamp.Time)
		logging.FromContext(ctx).Info("GitLabSource became ready after", zap.Duration("duration", duration))
		r.Recorder.Event(gitlab, corev1.EventTypeNormal, gitLabSourceReadinessChanged, fmt.Sprintf("GitLabSource %q became ready", gitlab.Name))
	}
	return result, err
}
