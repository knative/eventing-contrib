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
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"
	sourcesv1alpha1 "gitlab.com/triggermesh/gitlabsource/pkg/apis/sources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerAgentName = "gitlab-source-controller"
	finalizerName       = controllerAgentName
	raImageEnvVar       = "GL_RA_IMAGE"
)

var log = logf.Log.WithName("controller")
var receiveAdapterImage string

// Add creates a new GitLabSource Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	var defined bool
	receiveAdapterImage, defined = os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileGitLabSource{Client: mgr.GetClient(), scheme: mgr.GetScheme(), receiveAdapterImage: receiveAdapterImage}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("gitlabsource-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to GitLabSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.GitLabSource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &servingv1alpha1.Service{}}, &handler.EnqueueRequestForOwner{OwnerType: &servingv1alpha1.Service{}, IsController: true})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGitLabSource{}

// ReconcileGitLabSource reconciles a GitLabSource object
type ReconcileGitLabSource struct {
	client.Client
	scheme              *runtime.Scheme
	receiveAdapterImage string
}

// Reconcile reads that state of the cluster for a GitLabSource object and makes changes based on the state read
// and what is in the GitLabSource.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=sources.eventing.triggermesh.dev,resources=gitlabsources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sources.eventing.triggermesh.dev,resources=gitlabsources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=serving.knative.dev,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=eventing.knative.dev,resources=channels,verbs=get;list;watch
func (r *ReconcileGitLabSource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()

	log.Info("Reconciling " + request.NamespacedName.String())

	// Fetch the GitLabSource instance
	sourceOrg := &sourcesv1alpha1.GitLabSource{}
	err := r.Get(ctx, request.NamespacedName, sourceOrg)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// Finalizers ensure that controller gets chance to process gitlabsource  delete
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	source := sourceOrg.DeepCopyObject()
	var reconcileErr error
	if sourceOrg.ObjectMeta.DeletionTimestamp == nil {
		reconcileErr = r.reconcile(source.(*sourcesv1alpha1.GitLabSource))
	} else {
		if r.hasFinalizer(source.(*sourcesv1alpha1.GitLabSource)) {
			reconcileErr = r.finalize(source.(*sourcesv1alpha1.GitLabSource))
		}
	}
	if err := r.Update(ctx, source); err != nil {
		log.Error(err, "Failed to update")
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, reconcileErr
}

func getGitlabBaseUrl(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	baseurl := strings.TrimSuffix(projectUrl, projectName)
	return baseurl, nil
}

func getProjectName(projectUrl string) (string, error) {
	u, err := url.Parse(projectUrl)
	if err != nil {
		return "", err
	}
	projectName := u.Path[1:]
	return projectName, nil
}

func (r *ReconcileGitLabSource) reconcile(source *sourcesv1alpha1.GitLabSource) error {

	source.Status.InitializeConditions()

	ctx := context.TODO()
	hookOptions := projectHookOptions{}
	projectName, err := getProjectName(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}
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
		return err
	}
	hookOptions.secretToken, err = r.secretFrom(source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		return err
	}

	uri, err := r.getSinkURI(source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "%s", err)
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.getOwnedKnativeService(source)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ksvc = r.generateKnativeServiceObject(source, r.receiveAdapterImage)
			if err = controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
				return fmt.Errorf("Failed to create knative service for the gitlabsource: " + err.Error())
			}
			if err = r.Create(ctx, ksvc); err != nil {
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
	if source.Spec.SslVerify {
		hookOptions.url = "https://" + ksvc.Status.Domain
		hookOptions.EnableSSLVerification = true
	} else {
		hookOptions.url = "http://" + ksvc.Status.Domain
	}

	baseUrl, err := getGitlabBaseUrl(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the base url: " + err.Error())
	}
	gitlabClient := gitlabHookClient{}
	hookId, err := gitlabClient.Create(baseUrl, &hookOptions)
	if err != nil {
		return fmt.Errorf("Failed to create project hook: " + err.Error())
	}
	source.Status.Id = hookId
	r.addFinalizer(source)
	return nil
}

func (r *ReconcileGitLabSource) finalize(source *sourcesv1alpha1.GitLabSource) error {

	hookOptions := projectHookOptions{}
	projectName, err := getProjectName(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the project name: " + err.Error())
	}
	hookOptions.project = projectName
	hookOptions.id = source.Status.Id
	hookOptions.accessToken, err = r.secretFrom(source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		return err
	}

	baseUrl, err := getGitlabBaseUrl(source.Spec.ProjectUrl)
	if err != nil {
		return fmt.Errorf("Failed to process project url to get the base url: " + err.Error())
	}
	gitlabClient := gitlabHookClient{}
	err = gitlabClient.Delete(baseUrl, &hookOptions)
	if err != nil {
		return fmt.Errorf("Failed to delete project hook: " + err.Error())
	}
	r.removeFinalizer(source)
	return nil
}

func (r *ReconcileGitLabSource) secretFrom(namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	err := r.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)

	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf(`key "%s" not found in secret "%s"`, secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func (r *ReconcileGitLabSource) addFinalizer(source *sourcesv1alpha1.GitLabSource) {
	finalizers := sets.NewString(source.Finalizers...)
	finalizers.Insert(finalizerName)
	source.Finalizers = finalizers.List()
}

func (r *ReconcileGitLabSource) removeFinalizer(source *sourcesv1alpha1.GitLabSource) {
	finalizers := sets.NewString(source.Finalizers...)
	finalizers.Delete(finalizerName)
	source.Finalizers = finalizers.List()
}

func (r *ReconcileGitLabSource) hasFinalizer(source *sourcesv1alpha1.GitLabSource) bool {
	for _, finalizerStr := range source.Finalizers {
		if finalizerStr == finalizerName {
			return true
		}
	}
	return false
}

func (r *ReconcileGitLabSource) generateKnativeServiceObject(source *sourcesv1alpha1.GitLabSource, receiveAdapterImage string) *servingv1alpha1.Service {
	labels := map[string]string{
		"receive-adapter": "gitlab",
	}
	sinkURI := source.Status.SinkURI
	env := []corev1.EnvVar{
		{
			Name: "GITLAB_SECRET_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: source.Spec.SecretToken.SecretKeyRef,
			},
		},
	}
	containerArgs := []string{fmt.Sprintf("--sink=%s", sinkURI)}
	return &servingv1alpha1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", source.Name),
			Namespace:    source.Namespace,
			Labels:       labels,
		},
		Spec: servingv1alpha1.ServiceSpec{
			RunLatest: &servingv1alpha1.RunLatestType{
				Configuration: servingv1alpha1.ConfigurationSpec{
					RevisionTemplate: servingv1alpha1.RevisionTemplateSpec{
						Spec: servingv1alpha1.RevisionSpec{
							ServiceAccountName: source.Spec.ServiceAccountName,
							Container: corev1.Container{
								Image: receiveAdapterImage,
								Env:   env,
								Args:  containerArgs,
							},
						},
					},
				},
			},
		},
	}
}

func (r *ReconcileGitLabSource) getOwnedKnativeService(source *sourcesv1alpha1.GitLabSource) (*servingv1alpha1.Service, error) {
	ctx := context.TODO()
	list := &servingv1alpha1.ServiceList{}
	err := r.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
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
			return &ksvc, nil
		}
	}

	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

func (r *ReconcileGitLabSource) waitForKnativeServiceReady(source *sourcesv1alpha1.GitLabSource) (*servingv1alpha1.Service, error) {
	for attempts := 0; attempts < 4; attempts++ {
		ksvc, err := r.getOwnedKnativeService(source)
		if err != nil {
			return nil, err
		}
		routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
		receiveAdapterDomain := ksvc.Status.Domain
		if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
			return ksvc, nil
		}
		time.Sleep(2000 * time.Millisecond)
	}
	return nil, fmt.Errorf("Failed to get service to be in ready state")
}

func (r *ReconcileGitLabSource) getSinkURI(sink *corev1.ObjectReference, namespace string) (string, error) {

	ctx := context.TODO()

	if sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(sink.GroupVersionKind())
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: sink.Name}, u)
	if err != nil {
		return "", err
	}

	t := duckv1alpha1.AddressableType{}
	err = duck.FromUnstructured(u, &t)
	if err != nil {
		return "", fmt.Errorf("failed to deserialize sink: %v", err)
	}

	if t.Status.Address == nil {
		return "", fmt.Errorf("sink does not contain address")
	}

	if t.Status.Address.Hostname == "" {
		return "", fmt.Errorf("sink contains an empty hostname")
	}

	return fmt.Sprintf("http://%s/", t.Status.Address.Hostname), nil
}
