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

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReceiveAdapterArgs are the arguments needed to create an AWS SQS
// Source Receive Adapter. Every field is required.
type ReceiveAdapterArgs struct {
	Image   string
	Source  *v1alpha1.AwsSqsSource
	Labels  map[string]string
	SinkURI string
}

const (
	credsVolume    = "aws-credentials"
	credsMountPath = "/var/secrets/aws"
)

// MakeReceiveAdapter generates (but does not insert into K8s) the
// Receive Adapter Deployment for AWS SQS Sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	credsFile := fmt.Sprintf("%s/%s", credsMountPath, args.Source.Spec.AwsCredsSecret.Key)
	replicas := int32(1)
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    args.Source.Namespace,
			GenerateName: fmt.Sprintf("awssqs-%s-", args.Source.Name),
			Labels:       args.Labels,
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: args.Image,
							Env: []corev1.EnvVar{
								{
									Name:  "AWS_APPLICATION_CREDENTIALS",
									Value: credsFile,
								},
								{
									Name:  "AWS_SQS_URL",
									Value: args.Source.Spec.QueueURL,
								},
								{
									Name:  "SINK_URI",
									Value: args.SinkURI,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      credsVolume,
									MountPath: credsMountPath,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: credsVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: args.Source.Spec.AwsCredsSecret.Name,
								},
							},
						},
					},
				},
			},
		},
	}
}
