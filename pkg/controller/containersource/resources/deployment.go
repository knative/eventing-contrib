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

package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeDeployment(org *appsv1.Deployment, args *ContainerArguments) (*appsv1.Deployment, error) {
	containerArgs := []string(nil)
	if args != nil {
		containerArgs = args.Args
	}
	if !args.SinkInArgs {
		remote := fmt.Sprintf("--sink=%s", args.Sink)
		containerArgs = append(containerArgs, remote)
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.Name + "-",
			Namespace:    args.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { var i int32 = 1; return &i }(),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"source": args.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"source": args.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "source",
							Image:           args.Image,
							Args:            containerArgs,
							ImagePullPolicy: corev1.PullAlways,
						},
					},
				},
			},
		},
	}

	return deploy, nil
}
