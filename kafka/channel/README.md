# Apache Kafka Channels

Kafka channels are those backed by [Apache Kafka](http://kafka.apache.org/)
topics.

## Deployment steps

1. Setup [Knative Eventing](../../DEVELOPMENT.md)
1. Install an Apache Kafka cluster, if you have not done so already.

   For Kubernetes a simple installation is done using the
   [Strimzi Kafka Operator](http://strimzi.io). Its installation
   [guides](http://strimzi.io/quickstarts/) provide content for Kubernetes and
   Openshift.

   > Note: This `KafkaChannel` is not limited to Apache Kafka installations on
   > Kubernetes. It is also possible to use an off-cluster Apache Kafka
   > installation.

1. Now that Apache Kafka is installed, you need to configure the
   `bootstrapServers` value in the `config-kafka` ConfigMap, located inside the
   `config/400-kafka-config.yaml` file.

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: config-kafka
     namespace: knative-eventing
   data:
     # Broker URL. Replace this with the URLs for your kafka cluster,
     # which is in the format of my-cluster-kafka-bootstrap.my-kafka-namespace:9092.
     bootstrapServers: REPLACE_WITH_CLUSTER_URL
   ```

1. Apply the Kafka config:

   ```sh
   ko apply -f config
   ```

1. Create the `KafkaChannel` custom objects:

   ```yaml
   apiVersion: messaging.knative.dev/v1alpha1
   kind: KafkaChannel
   metadata:
     name: my-kafka-channel
   spec:
     numPartitions: 1
     replicationFactor: 1
   ```

   You can configure the number of partitions with `numPartitions`, as well as
   the replication factor with `replicationFactor`. If not set, both will
   default to `1`.

## Components

The major components are:

- Kafka Channel Controller
- Kafka Channel Dispatcher
- Kafka Webhook
- Kafka Config Map

The Kafka Channel Controller is located in one Pod:

```shell
kubectl get deployment -n knative-eventing kafka-ch-controller
```

The Kafka Channel Dispatcher receives and distributes all events to the
appropriate consumers:

```shell
kubectl get deployment -n knative-eventing kafka-ch-dispatcher
```

The Kafka Webhook is used to validate and set defaults to `KafkaChannel` custom
objects:

```shell
kubectl get deployment -n knative-eventing kafka-webhook
```

The Kafka Config Map is used to configure the `bootstrapServers` of your Apache
Kafka installation:

```shell
kubectl get configmap -n knative-eventing config-kafka
```

### Namespace Dispatchers

By default events are received and dispatched by a single cluster-scoped
dispatcher components. You can also specify whether events should be received
and dispatched by the dispatcher in the same namespace as the channel definition
by adding the `eventing.knative.dev/scope: namespace` annotation.

First, you need to create the configMap `config-kafka` in the same namespace as
the KafkaChannel.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-kafka
  namespace: <YOUR_NAMESPACE>
data:
  # Broker URL. Replace this with the URLs for your kafka cluster,
  # which is in the format of my-cluster-kafka-bootstrap.my-kafka-namespace:9092.
  bootstrapServers: REPLACE_WITH_CLUSTER_URL
```

> Note: the `bootstrapServers` value does not have to be the same as the one
> specified in `knative-eventing/config-kafka`.

Then create a KafkaChannel:

```yaml
apiVersion: messaging.knative.dev/v1alpha1
kind: KafkaChannel
metadata:
  name: my-kafka-channel
  namespace: <YOUR_NAMESPACE>
  annotations:
    eventing.knative.dev/scope: namespace
spec:
  numPartitions: 1
  replicationFactor: 1
```

The dispatcher is created in `<YOUR_NAMESPACE>`:

```sh
kubectl get deployment -n <YOUR_NAMESPACE> kafka-ch-dispatcher
```

Both cluster-scoped and namespace-scoped dispatcher can coexist. However once
the annotation is set (or not set), its value is immutable.
