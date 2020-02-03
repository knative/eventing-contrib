# Apache Kafka Channels

Kafka channels are those backed by [Apache Kafka](http://kafka.apache.org/)
topics.

## Deployment steps

1. Setup [Knative Eventing](../../DEVELOPMENT.md) Install an Apache Kafka
   cluster, if you have not done so already.

   - For Kubernetes a simple installation is done using the
     [Strimzi Kafka Operator](http://strimzi.io). Its installation
     [guides](http://strimzi.io/quickstarts/) provide content for Kubernetes and
     Openshift.

   > Note: This `KafkaChannel` is not limited to Apache Kafka installations on
   > Kubernetes. It is also possible to use an off-cluster Apache Kafka
   > installation.

1. Decide on where you want the Kafka Channel Dispatcher to be installed,
   either in `knative-eventing` (Cluster) or in the same namespace as KafkaChannel
   objects (Namespace).

1. Now that Apache Kafka is installed, you need to configure the
   `bootstrapServers` value in the `config-kafka` ConfigMap, located inside the
   `config/400-kafka-config.yaml`.

   * **Cluster**: make sure `namespace` is set to `knative-eventing`:

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

   * **Namespace**: set `namespace` to where you are planning to create `KafkaChannel` objects. You can also omit it:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: config-kafka
   data:
     # Broker URL. Replace this with the URLs for your kafka cluster,
     # which is in the format of my-cluster-kafka-bootstrap.my-kafka-namespace:9092.
     bootstrapServers: REPLACE_WITH_CLUSTER_URL
   ```

1. Apply the Kafka config:

    * Either **Cluster**:
   ```
   ko apply -f config
   ```

    * or **Namespace**:
   ```
   ko apply -f config/namespace
   ```

   Do not apply both!

1. Create the `KafkaChannel` custom objects:

   ```yaml
   apiVersion: messaging.knative.dev/v1alpha1
   kind: KafkaChannel
   metadata:
     name: my-kafka-channel
   spec:
     numPartitions: 1
     replicationFactor: 3
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

* **Cluster**:
```shell
kubectl get deployment -n knative-eventing kafka-ch-dispatcher
```

* **Namespace**:
```shell
kubectl get deployment kafka-ch-dispatcher
```

The Kafka Webhook is used to validate and set defaults to `KafkaChannel` custom
objects:

```shell
kubectl get deployment -n knative-eventing kafka-webhook
```

The Kafka Config Map is used to configure the `bootstrapServers` of your Apache
Kafka installation:

* **Cluster**:

```shell
kubectl get configmap -n knative-eventing config-kafka
```

* **Namespace**:
```shell
kubectl get configmap   config-kafka
```
