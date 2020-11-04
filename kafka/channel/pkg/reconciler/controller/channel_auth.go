package controller

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	messagingv1beta1 "knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
)

//type KafkaTLSAuthConfig struct {
//	Cacert   string
//	Usercert string
//	Userkey  string
//}

func (r *Reconciler) GetKafkaAuthData(ctx context.Context, kc *messagingv1beta1.KafkaChannel)  (map[string]string, error) {

	// if present, we exact the TLS data
	if kc.Spec.Auth != nil {
		if kc.Spec.Auth.Namespace == "" || kc.Spec.Auth.Name == "" {
			return nil, errors.New("kafkachannel.spec.auth.[name, namespace] are required") // TODO: throwing an error here is wrong. This should set a custom condition.
		}

		gvk := fmt.Sprintf("%s.%s", kc.Spec.Auth.Kind, kc.Spec.Auth.APIVersion)

		switch gvk {
		case "Secret.v1":
			config, err := r.kafkaAuthDataFromSecret(ctx, kc.Spec.Auth)
			utils.CopySecret(r.KubeClientSet.CoreV1(), kc.Spec.Auth.Namespace, kc.Spec.Auth.Name, r.systemNamespace)
			if err != nil {
				logging.FromContext(ctx).Errorw("Unable to load KafkaAuth data from KafkaChannel.Spec.Auth as v1:Secret.", zap.Error(err))
			}
			return config, nil
		default:
			return nil, errors.New("KafkaChannel.Spec.Auth configuration not supported, only [kind: Secret, apiVersion: v1]") // TODO: throwing an error here is wrong. This should set a custom condition.
		}
	}
	return nil, nil
}


//func (r *Reconciler) kafkaAuthDataFromSecret(ctx context.Context, ref *duckv1.KReference) (*KafkaTLSAuthConfig, error) {
func (r *Reconciler) kafkaAuthDataFromSecret(ctx context.Context, ref *duckv1.KReference) (map[string]string, error) {
	secret, err := r.KubeClientSet.CoreV1().Secrets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("Secret '%s' not found %s", ref.Name)
	}

	tlsConfig := make(map[string]string)
	tlsConfig["Cacert"] = string(secret.Data["ca.crt"])
	tlsConfig["Usercert"] = string(secret.Data["user.crt"])
	tlsConfig["Userkey"] = string(secret.Data["user.key"])

	//config := &KafkaTLSAuthConfig{}
	//config.Cacert = string(secret.Data["ca.crt"])
	//config.Usercert = string(secret.Data["user.crt"])
	//config.Userkey = string(secret.Data["user.key"])

	r.kafkaTls = tlsConfig
	return tlsConfig, nil
}
