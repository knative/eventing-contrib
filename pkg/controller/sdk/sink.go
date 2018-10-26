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

package sdk

import (
	"context"
	"fmt"
	duckapis "github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

const (
	tryTargetable = true
)

type SinkUri struct{}

func (s *SinkUri) Get(ctx context.Context, namespace string, sink *corev1.ObjectReference, dynamicClient dynamic.Interface) (string, error) {
	logger := logging.FromContext(ctx)

	// check to see if the source has provided a sink ref in the spec. Lets look for it.

	if sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	obj, err := s.fetchObjectReference(ctx, namespace, sink, dynamicClient)
	if err != nil {
		logger.Warnf("Failed to fetch sink target %+v: %s", sink, zap.Error(err))
		return "", err
	}
	t := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(obj, &t)
	if err != nil {
		logger.Warnf("Failed to deserialize sink: %s", zap.Error(err))
		return "", err
	}

	if t.Status.Sinkable != nil {
		return fmt.Sprintf("http://%s/", t.Status.Sinkable.DomainInternal), nil
	}

	// for now we will try again as a targetable.
	if tryTargetable {
		t := duckv1alpha1.Target{}
		err = duck.FromUnstructured(obj, &t)
		if err != nil {
			logger.Warnf("Failed to deserialize targetable: %s", zap.Error(err))
			return "", err
		}

		if t.Status.Targetable != nil {
			return fmt.Sprintf("http://%s/", t.Status.Targetable.DomainInternal), nil
		}
	}

	return "", fmt.Errorf("sink does not contain sinkable")
}

// fetchObjectReference fetches an object based on ObjectReference.
func (s *SinkUri) fetchObjectReference(ctx context.Context, namespace string, ref *corev1.ObjectReference, dynamicClient dynamic.Interface) (duck.Marshalable, error) {
	logger := logging.FromContext(ctx)

	resourceClient, err := s.CreateResourceInterface(namespace, ref, dynamicClient)
	if err != nil {
		logger.Warnf("failed to create dynamic client resource: %v", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func (s *SinkUri) CreateResourceInterface(namespace string, ref *corev1.ObjectReference, dynamicClient dynamic.Interface) (dynamic.ResourceInterface, error) {
	rc := dynamicClient.Resource(duckapis.KindToResource(ref.GroupVersionKind()))
	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil
}
