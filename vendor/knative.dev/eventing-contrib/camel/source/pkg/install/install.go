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

package install

import (
	"context"
	"log"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	controller "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	installationCheckInterval = 30 * time.Second
	requestTimeout            = 25 * time.Second
)

// EnsureCamelKInstalled periodically checks Camel K cluster resources installation and returns only when they are found
func EnsureCamelKInstalled(cfg *rest.Config) {
	installed := false
	var err error
	var c controller.Client

	options := controller.Options{
		Scheme: clientscheme.Scheme,
	}
	for c, err = controller.New(cfg, options); err != nil; {
		log.Printf("Unable to create client to check Camel K installation: %v", zap.Error(err))
		time.Sleep(installationCheckInterval)
	}

	for !installed {
		if installed, err = IsCamelKInstalled(c); err != nil {
			log.Printf("Failed to check Camel K installation: %v", zap.Error(err))
			time.Sleep(installationCheckInterval)
		} else if !installed {
			log.Printf("Camel K cluster resources not found in the cluster. Follow installation instructions at: https://github.com/apache/camel-k")
			time.Sleep(installationCheckInterval)
		}
	}
}

// IsCamelKInstalled checks if Camel K cluster resources are installed in the current cluster
func IsCamelKInstalled(c controller.Client) (bool, error) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(requestTimeout))
	defer cancel()
	opts := controller.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"app": "camel-k"}),
	}
	list := v1.ClusterRoleList{}

	err := c.List(ctx, &list, &opts)
	if err != nil {
		return false, err
	}
	return len(list.Items) > 0, nil
}
