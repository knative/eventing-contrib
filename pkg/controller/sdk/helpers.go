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
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

// fetchObjectReference fetches an object based on ObjectReference.
func FetchObjectReference(ctx context.Context, dc dynamic.Interface, namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	logger := logging.FromContext(ctx)

	resourceClient, err := CreateResourceInterface(dc, namespace, ref)
	if err != nil {
		logger.Warnf("failed to create dynamic client resource: %v", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func CreateResourceInterface(dc dynamic.Interface, namespace string, ref *corev1.ObjectReference) (dynamic.ResourceInterface, error) {
	rc := dc.Resource(duckapis.KindToResource(ref.GroupVersionKind()))
	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil
}
