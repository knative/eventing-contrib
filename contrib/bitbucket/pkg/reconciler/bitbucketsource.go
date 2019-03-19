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

package reconciler

import (
	"context"
	"fmt"
	"os"
	"strings"

	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/bitbucket/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/contrib/bitbucket/pkg/reconciler/resources"
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
	controllerAgentName = "bitbucket-source-controller"
	raImageEnvVar       = "BB_RA_IMAGE"
	finalizerName       = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	finalizeFailed         = "FinalizeFailed"
	sinkNotFound           = "SinkNotFound"
	consumerKeyNotFound    = "ConsumerKeyNotFound"
	consumerSecretNotFound = "ConsumerSecretNotFound"
	ownerAndRepoInvalid    = "OwnerAndRepositoryInvalid"
	svcDomainNotFound      = "ServiceDomainNotFound"
	webhookCreated         = "WebHookCreated"
	webhookCreateFailed    = "WebHookCreateFailed"
	serviceCreated         = "ServiceCreated"
	serviceCreateFailed    = "ServiceCreateFailed"
)

type webhookArgs struct {
	source         *sourcesv1alpha1.BitBucketSource
	domain         string
	consumerKey    string
	consumerSecret string
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

	consumerKey, consumerSecret, err := r.secretsFrom(ctx, source)
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
		// Returning nil on purpose as we will wait until the next reconciliation process is triggered.
		// Noticed that when triggering it right away (or after few seconds), it always fails to
		// update the status after we create a webhook, thus two webhooks are created.
		return nil
	}
	source.Status.MarkService()

	hookUUID, err := r.reconcileWebHook(ctx, source, domain, consumerKey, consumerSecret)
	if err != nil {
		return err
	}
	source.Status.MarkWebHook(hookUUID)

	// If the webhook was created, but there's a failure updating its status in the sdk/reconciler.go,
	// we might end up creating another webhook. We record the webhook creation event so that it can be
	// removed manually by the operator.
	// TODO any other way to avoid this?

	return nil
}

func (r *reconciler) finalize(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) error {
	// Always remove the finalizer. If there's a failure cleaning up, an event
	// will be recorded allowing the webhook to be removed manually by the
	// operator.
	r.removeFinalizer(source)

	// If a webhook was created, try to delete it.
	if source.Status.WebhookUUIDKey != "" {
		// Get tokens.
		consumerKey, consumerSecret, err := r.secretsFrom(ctx, source)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, finalizeFailed, "Could not delete webhook %q: %v", source.Status.WebhookUUIDKey, err)
			return err
		}

		// Delete the webhook using the access and secret tokens, and the stored webhook UUID in the source.Status.
		webhookArgs := &webhookArgs{
			source:         source,
			consumerKey:    consumerKey,
			consumerSecret: consumerSecret,
		}
		err = r.deleteWebhook(ctx, webhookArgs)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, finalizeFailed, "Could not delete webhook %q: %v", source.Status.WebhookUUIDKey, err)
			return err
		}
		// Clear the UUID.
		source.Status.WebhookUUIDKey = ""
	}
	return nil
}

func (r *reconciler) domainFrom(ksvc *servingv1alpha1.Service, source *sourcesv1alpha1.BitBucketSource) (string, error) {
	routeCondition := ksvc.Status.GetCondition(servingv1alpha1.ServiceConditionRoutesReady)
	receiveAdapterDomain := ksvc.Status.Domain
	if routeCondition != nil && routeCondition.Status == corev1.ConditionTrue && receiveAdapterDomain != "" {
		return receiveAdapterDomain, nil
	}
	r.recorder.Eventf(source, corev1.EventTypeWarning, svcDomainNotFound, "Could not find domain for service %q", ksvc.Name)
	err := fmt.Errorf("domain not found for svc %q", ksvc.Name)
	source.Status.MarkNoService(svcDomainNotFound, "%s", err)
	return "", err
}

func (r *reconciler) reconcileWebHook(ctx context.Context, source *sourcesv1alpha1.BitBucketSource, domain, consumerKey, consumerSecret string) (string, error) {
	// If webhook doesn't exist, then create it.
	if source.Status.WebhookUUIDKey == "" {
		// Add a finalizer to be able to delete it.
		r.addFinalizer(source)

		webhookArgs := &webhookArgs{
			source:         source,
			domain:         domain,
			consumerKey:    consumerKey,
			consumerSecret: consumerSecret,
		}
		hookID, err := r.createWebhook(ctx, webhookArgs)
		if err != nil {
			r.recorder.Eventf(source, corev1.EventTypeWarning, webhookCreateFailed, "Could not create webhook: %v", err)
			source.Status.MarkNoWebHook(webhookCreateFailed, "%s", err)
			return "", err
		}
		source.Status.WebhookUUIDKey = hookID
		r.recorder.Eventf(source, corev1.EventTypeNormal, webhookCreated, "Created webhook: %q", hookID)
	}
	return source.Status.WebhookUUIDKey, nil
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
			r.recorder.Eventf(source, corev1.EventTypeWarning, serviceCreateFailed, "Could not create service: %v", err)
			source.Status.MarkNoService(serviceCreateFailed, "%s", err)
			return nil, err
		}
		r.recorder.Eventf(source, corev1.EventTypeNormal, serviceCreated, "Created service: %q", ksvc.Name)
		return ksvc, nil
	} else if err != nil {
		return nil, err
	}

	return current, nil
}

func (r *reconciler) createWebhook(ctx context.Context, args *webhookArgs) (string, error) {

	owner, repo, err := r.ownerRepoFrom(args.source)
	if err != nil {
		return "", err
	}

	hookOptions := &webhookOptions{
		consumerKey:    args.consumerKey,
		consumerSecret: args.consumerSecret,
		domain:         args.domain,
		owner:          owner,
		repo:           repo,
		events:         args.source.Spec.EventTypes,
	}
	hookUUID, err := r.webhookClient.Create(ctx, hookOptions)
	if err != nil {
		return "", err
	}
	return hookUUID, nil
}

func (r *reconciler) deleteWebhook(ctx context.Context, args *webhookArgs) error {
	owner, repo, err := r.ownerRepoFrom(args.source)
	if err != nil {
		return err
	}

	hookOptions := &webhookOptions{
		uuid:           args.source.Status.WebhookUUIDKey,
		owner:          owner,
		repo:           repo,
		consumerKey:    args.consumerKey,
		consumerSecret: args.consumerSecret,
	}
	return r.webhookClient.Delete(ctx, hookOptions)
}

func (r *reconciler) sinkURIFrom(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (string, error) {
	uri, err := sinks.GetSinkURI(ctx, r.client, source.Spec.Sink, source.Namespace)
	if err != nil {
		r.recorder.Eventf(source, corev1.EventTypeWarning, sinkNotFound, "Could not find sink: %v", err)
		source.Status.MarkNoSink(sinkNotFound, "%s", err)
		return "", err
	}
	return uri, err
}

func (r *reconciler) secretsFrom(ctx context.Context, source *sourcesv1alpha1.BitBucketSource) (string, string, error) {

	consumerKey, err := r.secretFrom(ctx, source.Namespace, source.Spec.ConsumerKey.SecretKeyRef)
	if err != nil {
		r.recorder.Eventf(source, corev1.EventTypeWarning, consumerKeyNotFound, "Could not find consumer key: %v", err)
		source.Status.MarkNoSecrets(consumerKeyNotFound, "%s", err)
		return "", "", err
	}
	consumerSecret, err := r.secretFrom(ctx, source.Namespace, source.Spec.ConsumerSecret.SecretKeyRef)
	if err != nil {
		r.recorder.Eventf(source, corev1.EventTypeWarning, consumerSecretNotFound, "Could not find consumer secret: %v", err)
		source.Status.MarkNoSecrets(consumerSecretNotFound, "%s", err)
		return "", "", err
	}

	return consumerKey, consumerSecret, err
}

func (r *reconciler) secretFrom(ctx context.Context, namespace string, secretKeySelector *corev1.SecretKeySelector) (string, error) {
	if secretKeySelector == nil {
		return "", fmt.Errorf("nil secret key selector")
	}

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

func (r *reconciler) ownerRepoFrom(source *sourcesv1alpha1.BitBucketSource) (string, string, error) {
	ownerAndRepository := source.Spec.OwnerAndRepository
	components := strings.Split(ownerAndRepository, "/")
	if len(components) > 2 {
		err := fmt.Errorf("ownerAndRepository is malformatted, expected 'owner/repository' but found %q", ownerAndRepository)
		r.recorder.Eventf(source, corev1.EventTypeWarning, ownerAndRepoInvalid, "Invalid owner and repository: %v", err)
		return "", "", err
	}
	owner := components[0]
	if len(owner) == 0 && len(components) > 1 {
		err := fmt.Errorf("owner is empty, expected 'owner/repository' but found %q", ownerAndRepository)
		r.recorder.Eventf(source, corev1.EventTypeWarning, ownerAndRepoInvalid, "Invalid owner and repository: %v", err)
		return "", "", err
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
