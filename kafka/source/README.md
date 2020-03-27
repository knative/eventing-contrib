# Apache Kafka - Source

The Apache Kafka Event source enables Knative Eventing integration with Apache
Kafka. When a message is produced to Apache Kafka, the Apache Kafka Event Source
will consume the produced message and post that message to the corresponding
event sink.

## Deployment steps

1. Setup [Knative Eventing](../../DEVELOPMENT.md)
1. If not done already, install an Apache Kafka cluster!

   - For Kubernetes a simple installation is done using the
     [Strimzi Kafka Operator](http://strimzi.io). Its installation
     [guides](http://strimzi.io/quickstarts/) provide content for Kubernetes and
     Openshift.

   > Note: The `KafkaSource` is not limited to Apache Kafka installations on
   > Kubernetes. It is also possible to use an off-cluster Apache Kafka
   > installation.

1. Now that Apache Kafka is installed, apply the `KafkaSource` config:

   ```
   ko apply -f config/
   ```

1. Create the `KafkaSource` custom objects, by configuring the required
   `consumerGroup`, `bootstrapServers` and `topics` values on the CR file of
   your source. Below is an example:

   ```yaml
   apiVersion: sources.knative.dev/v1alpha1
   kind: KafkaSource
   metadata:
     name: kafka-source
   spec:
     consumerGroup: knative-group
     # Broker URL. Replace this with the URLs for your kafka cluster,
     # which is in the format of my-cluster-kafka-bootstrap.my-kafka-namespace:9092.
     bootstrapServers: REPLACE_WITH_CLUSTER_URL
     topics: knative-demo-topic
     sink:
       ref:
         apiVersion: serving.knative.dev/v1alpha1
         kind: Service
         name: event-display
   ```

## Example

A more detailed example of the `KafkaSource` can be found in the
[Knative documentation](https://knative.dev/docs/eventing/samples/).

## Experimental KEDA support in Kafka Event Source

Warning: this is *experimental* and may be changed in future. Should not be used in production. This is mainly for discussion and evolving scaling in Knative eventing.

The code is using Unstructured and also imported KEDA API - this is for discussion which version should be used (right now only Unstructured is fully implemented).
KEDA to provide client-go support discussion #494 <https://github.com/kedacore/keda/issues/494>

### Install KEDA

To install the version I used for the experiment (the latest version from master should work too):

```bash
git clone https://github.com/kedacore/keda.git
cd keda
git checkout v1.3.0
```

Then follow [KEDA setup instructions](https://keda.sh/deploy/) to deploy using YAML:

```bash
kubectl apply -f deploy/crds/keda.k8s.io_scaledobjects_crd.yaml
kubectl apply -f deploy/crds/keda.k8s.io_triggerauthentications_crd.yaml
kubectl apply -f deploy/
```

If worked expected to have in keda namespace:

```bash
kubectl get pods -n keda
NAME                                      READY   STATUS    RESTARTS   AGE
keda-metrics-apiserver-5bb8b6664c-vrhrj   1/1     Running   0          38s
keda-operator-865ccfb9d9-xd85w            1/1     Running   0          39s
```

### Run Kafka Source Controller

Install expermiental Kafka source controller with KEDA support from source code:

```bash
export KO_DOCKER_REPO=...
ko apply -f kafka/source/config/
```

#### Local testing

```bash
go run kafka/source/cmd/controller/main.go -kubeconfig $KUBECONFIG
```

### Create Kafka Source that uses KEDA

To use KEDA the YAML file must have one of autoscaling annotations (such as minScale).

Warning: temporary limitation - do not give KafkaSource name longer than 4 characters as KEDA will add prefix to deployment name that already has UUID appended HAP can not be created if its name exceed 64 character name limit (keda-hpa-kafkasource-SO_NAME-999036a8-af6e-4431-b671-d052842dddf1). For more details see https://github.com/kedacore/keda/issues/704

Example:

```yaml
apiVersion: sources.knative.dev/v1alpha1
kind: KafkaSource
metadata:
  name: kn1
  annotations:
    autoscaling.knative.dev/minScale: "0"
    autoscaling.knative.dev/maxScale: "10"
    autoscaling.knative.dev/class: keda.autoscaling.knative.dev
    keda.autoscaling.knative.dev/pollingInterval: "2"
    keda.autoscaling.knative.dev/cooldownPeriod: "15"
spec:
  bootstrapServers: my-cluster-kafka-bootstrap.kafka:9092 #note the .kafka in URL for namespace
  consumerGroup: knative-demo-kafka-keda-src1
  topics: knative-demo-topic
  sink:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: event-display
```

To verify that Kafka source is using KEDA retrieve the scaled object created by Kafka source:

```bash
⇒ kubectl get scaledobjects.keda.k8s.io
NAME   DEPLOYMENT                                             TRIGGERS   AGE
kn1    kafkasource-kn1-0e12266a-93c2-48ee-8d8d-3b6ffbe9d18f   kafka      26m
⇒  kubectl get horizontalpodautoscalers.autoscaling
NAME                                                            REFERENCE                                                         TARGETS              MINPODS   MAXPODS   REPLICAS   AGE
keda-hpa-kafkasource-kn1-0e12266a-93c2-48ee-8d8d-3b6ffbe9d18f   Deployment/kafkasource-kn1-0e12266a-93c2-48ee-8d8d-3b6ffbe9d18f   <unknown>/10 (avg)   1         10        0          26m
```
