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

package bitbucketsouce

import (
	"context"
	"fmt"
	"os"
	"strings"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/sdk"
	"github.com/knative/eventing-sources/pkg/controller/sinks"
	"github.com/knative/eventing-sources/pkg/reconciler/bitbucketsource/resources"
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
	controllerAgentName = "bitbucket-source-controller"
	raImageEnvVar       = "BB_RA_IMAGE"
	finalizerName       = controllerAgentName
)

type webhookArgs struct {
	source      *sourcesv1alpha1.BitBucketSource
	domain      string
	accessToken string
	secretToken string
}

// Add creates a new BitBucketSource Controller and adds it to the
// Manager with default RBAC. The Manager will set fields on the
// Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &sourcesv1alpha1.BitBucketSource{},
		Owns:      []runtime.Object{&servingv1alpha1.Service{}},
		Reconciler: &reconciler{
			recorder:            mgr.GetRecorder(controllerAgentName),
			scheme:              mgr.GetScheme(),
			receiveAdapterImage: receiveAdapterImage,
			webhookClient:       bitBucketWebhookClient{},
		},
	}

	return p.Add(mgr)
}

// reconciler reconciles a BitBucketSource object.
type reconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	recorder            record.EventRecorder
	receiveAdapterImage string
	webhookClient       webhookClient
}

// Reconcile reads that state of the cluster for a BitBucketSource
// object and makes changes based on the state read and what is in the
// BitBucketSource.Spec.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	source, ok := object.(*sourcesv1alpha1.BitBucketSource)
	if !ok {
		logger.Errorf("could not find BitBucket source %v", object)
		return nil
	}

	// See if the source has been deleted.
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

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) error {
	source.Status.InitializeConditions()

	accessToken, secretToken, err := r.secretsFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSecrets()

	uri, err := r.sinkURIFrom(ctx, source)
	if err != nil {
		return err
	}
	source.Status.MarkSink(uri)

	ksvc, err := r.reconcileService(ctx, source)
	if err != nil {
		return err
	}

	domain, err := r.domainFrom(ksvc, source)
	if err != nil {
		return err
	}
	source.Status.MarkService()

	hookUUID, err := r.reconcileWebHook(ctx, source, domain, accessToken, secretToken)
	if err != nil {
		return err
	}
	source.Status.MarkWebHook(hookUUID)

	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) error {
	// Always remove the finalizer. If there's a failure cleaning up, an event
	// will be recorded allowing the webhook to be removed manually by the
	// operator.
	r.removeFinalizer(source)

	// If a webhook was created, try to delete it.
	if source.Status.IsWebHookReady() {
		// Get tokens.
		accessToken, secretToken, err := r.secretsFrom(ctx, source)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FinalizeFailed", "Could not delete webhook %q: %v", source.Status.WebhookUUIDKey, err)
			return err
		}

		// Delete the webhook using the access and secret tokens, and the stored webhook UUID in the source.Status.
		webhookArgs := &webhookArgs{
			source:      source,
			accessToken: accessToken,
			secretToken: secretToken,
		}
		err = r.deleteWebhook(ctx, webhookArgs)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "FinalizeFailed", "Could not delete webhook %q: %v", source.Status.WebhookUUIDKey, err)
			return err
		}
		source.Status.MarkNoWebHook("WebHookDeleted", "%s", source.Status.WebhookUUIDKey)
	}
	return nil
}

func (r *reconciler) domainFrom(ksvc *servingv1alpha1.Service, source *sourcesv1alpha1.BitBucketSource) (string, error) {
	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
	receiveAdapterDomain := ksvc.Status.Domain
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
		return receiveAdapterDomain, nil
	}
	err := fmt.Errorf("domain not found for service %q", ksvc.Name)
	source.Status.MarkNoService("DomainNotFound", "%s", err)
	r.recorder.Eventf(ksvc, corev1.EventTypeWarning, "DomainNotFound", "Could not find domain for service %q", ksvc.Name)
	return "", err
}

func (r *reconciler) reconcileWebHook(ctx context.Context, source *sourcesv1alpha1.BitBucketSource, domain, accessToken, secretToken string) (string, error) {
	// If webhook doesn't exist, then create it.
	if !source.Status.IsWebHookReady() {
		// Add a finalizer to be able to delete it.
		r.addFinalizer(source)

		webhookArgs := webhookArgs{
			source:      source,
			domain:      domain,
			accessToken: accessToken,
			secretToken: secretToken,
		}
		hookID, err := r.createWebhook(ctx, webhookArgs)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, "WebHookCreateFailed", "Could not create webhook: %v", err)
			source.Status.MarkNoWebHook("WebHookCreateFailed", "%s", err)
			return "", err
		}
		return hookID, nil
	} else {
		return source.Status.WebhookUUIDKey, nil
	}
}

func (r *reconciler) reconcileService(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (*servingv1alpha1.Service, error) {
	current, err := r.getService(ctx, source)

	// If the resource doesn't exist, we'll create it.
	if apierrors.IsNotFound(err) {
		ksvc, err := r.newService(source)
		if err != nil {
			return nil, err
		}
		err = r.client.Create(ctx, ksvc)
		if err != nil {
			return nil, err
		}
		r.recorder.Eventf(source, corev1.EventTypeNormal, "ServiceCreated", "Created service %q", ksvc.Name)
		return ksvc, nil
	} else if err != nil {
		return nil, err
	}

	return current, nil
}

func (r *reconciler) createWebhook(ctx context.Context, args webhookArgs) (string, error) {

	logger := logging.FromContext(ctx)

	logger.Info("Creating BitBucket WebHook")

	owner, repo, err := ownerRepoFrom(args.source.Spec.OwnerAndRepository)
	if err != nil {
		return "", err
	}

	hookOptions := &webhookOptions{
		accessToken: args.accessToken,
		secretToken: args.secretToken,
		domain:      args.domain,
		owner:       owner,
		repo:        repo,
		events:      args.source.Spec.EventTypes,
	}
	hookUUID, err := r.webhookClient.Create(ctx, hookOptions)
	if err != nil {
		return "", fmt.Errorf("failed to create BitBucket WebHook: %v", err)
	}
	logger.Infof("Created BitBucket WebHook %s", hookUUID)
	return hookUUID, nil
}

func (r *reconciler) deleteWebhook(ctx context.Context, args *webhookArgs) error {
	logger := logging.FromContext(ctx)

	logger.Infof("Deleting BitBucket WebHook %q", args.source.Status.WebhookUUIDKey)

	owner, repo, err := ownerRepoFrom(args.source.Spec.OwnerAndRepository)
	if err != nil {
		return err
	}

	hookOptions := &webhookOptions{
		uuid:        args.source.Status.WebhookUUIDKey,
		owner:       owner,
		repo:        repo,
		accessToken: args.accessToken,
		secretToken: args.secretToken,
	}
	err = r.webhookClient.Delete(ctx, hookOptions)
	if err != nil {
		return fmt.Errorf("failed to delete BitBucket WebHook: %v", err)
	}
	logger.Infof("Deleted BitBucket WebHook %s", args.source.Status.WebhookUUIDKey)
	return nil
}

func (r *reconciler) sinkURIFrom(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (string, error) {
	uri, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		source.Status.MarkNoSink("SinkNotFound", "%s", err)
		r.recorder.Eventf(source, corev1.EventTypeWarning, "SinkNotFound", "Could not find sink: %v", err)
		return "", err
	}
	return uri, err
}

func (r *reconciler) secretsFrom(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (string, string, error) {

	accessToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.AccessToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("AccessTokenNotFound", "%s", err)
		r.recorder.Eventf(source, corev1.EventTypeWarning, "AccessTokenNotFound", "Could not find access token: %v", err)
		return "", "", err
	}
	secretToken, err := r.secretFrom(ctx, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		source.Status.MarkNoSecrets("SecretTokenNotFound", "%s", err)
		r.recorder.Eventf(source, corev1.EventTypeWarning, "SecretTokenNotFound", "Could not find secret token: %v", err)
		return "", "", err
	}

	return accessToken, secretToken, err
}

func (r *reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: secretKeySelector.Name}, secret)
	if err != nil {
		return "", err
	}
	secretVal, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %q", secretKeySelector.Key, secretKeySelector.Name)
	}
	return string(secretVal), nil
}

func ownerRepoFrom(ownerAndRepository string) (string, string, error) {
	components := strings.Split(ownerAndRepository, "/")
	if len(components) > 2 {
		return "", "", fmt.Errorf("ownerAndRepository is malformatted, expected 'owner/repository' but found %q", ownerAndRepository)
	}
	owner := components[0]
	if len(owner) == 0 && len(components) > 1 {
		return "", "", fmt.Errorf("owner is empty, expected 'owner/repository' but found %q", ownerAndRepository)
	}
	repo := ""
	if len(components) > 1 {
		repo = components[1]
	}

	return owner, repo, nil
}

func (r *reconciler) getService(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (*servingv1alpha1.Service, error) {
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
			return &ksvc, nil
		}
	}
	return nil, apierrors.NewNotFound(servingv1alpha1.Resource("services"), "")
}

func (r *reconciler) newService(source *sourcesv1alpha1.BitBucketSource) (*servingv1alpha1.Service, error) {
	ksvc := resources.MakeService(source, r.receiveAdapterImage)
	if err := controllerutil.SetControllerReference(source, ksvc, r.scheme); err != nil {
		return nil, err
	}
	return ksvc, nil
}

func (r *reconciler) addFinalizer(s *sourcesv1alpha1.BitBucketSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Insert(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(s *sourcesv1alpha1.BitBucketSource) {
	finalizers := sets.NewString(s.Finalizers...)
	finalizers.Delete(finalizerName)
	s.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
