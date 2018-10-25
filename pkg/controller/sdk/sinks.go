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
	"k8s.io/client-go/dynamic"

	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

func GetSinkUri(ctx context.Context, dc dynamic.Interface, sink *corev1.ObjectReference, namespace string) (string, error) {
	logger := logging.FromContext(ctx)

	// check to see if the source has provided a sink ref in the spec. Lets look for it.

	if sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	obj, err := FetchObjectReference(ctx, dc, namespace, sink)
	if err != nil {
		logger.Warnf("failed to fetch sink target %+v: %s", sink, zap.Error(err))
		return "", err
	}
	t := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(obj, &t)
	if err != nil {
		logger.Warnf("failed to deserialize sink: %s", zap.Error(err))
		return "", err
	}

	if t.Status.Sinkable != nil {
		return fmt.Sprintf("http://%s/", t.Status.Sinkable.DomainInternal), nil
	}

	return "", fmt.Errorf("sink does not contain sinkable")
}
