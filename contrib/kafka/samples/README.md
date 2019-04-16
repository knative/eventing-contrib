# Apache Kafka - Source

The Apache Kafka Event source enables Knative Eventing integration with Apache
Kafka. When a message is produced to Apache Kafka, the Apache Kafka Event Source
will consume the produced message and post that message to the corresponding
event sink.

This sample demonstrates how to configure, deploy, and use the Apache Kafka
Event Source with a Knative Service.

For Kubernetes, a simple Apache Kafka installation can be done with Strimzi,
check out their [Quickstart](https://strimzi.io/quickstarts/) for both Minikube
and Openshift guides. You can also install Kafka on the host.

## Build and Deploy Steps

### Prerequisites

1. An existing instance of Apache Kafka must be running to use the Apache Kafka
   Event Source.
   - In order to consume and produce messages, a topic must be created on the
     Apache Kafka instance.
   - A list of bootstrap servers corresponding to Apache Kafka instance must be
     obtained.
2. Install the `ko` CLI for building and deploying purposes.
   ```
   go get github.com/google/ko/cmd/ko
   ```
3. A container registry, such as a Docker Hub account, is required.
   - Export the `KO_DOCKER_REPO` environment variable with a value denoting the
     container registry to use.
     ```
     export KO_DOCKER_REPO="docker.io/YOUR_REPO"
     ```

### Build and Deployment

The following steps build and deploy the Apache Kafka Event Controller, Source,
and an Event Display Service.

- Assuming current working directory is the project root `eventing-sources`.

#### Apache Kafka Event Controller

1. Build the Apache Kafka Event Controller and configure a Service Account,
   Cluster Role, Controller, and Source.
   ```
   $ ko apply -f contrib/kafka/config
   ...
   serviceaccount/kafka-controller-manager created
   clusterrole.rbac.authorization.k8s.io/eventing-sources-kafka-controller created
   clusterrolebinding.rbac.authorization.k8s.io/eventing-sources-kafka-controller created
   customresourcedefinition.apiextensions.k8s.io/kafkasources.sources.eventing.knative.dev configured
   service/kafka-controller created
   statefulset.apps/kafka-controller-manager created
   ```
2. Check that the `kafka-controller-manager-0` pod is running.
   ```
   $ kubectl get pods -n knative-sources
   NAME                         READY     STATUS    RESTARTS   AGE
   kafka-controller-manager-0   1/1       Running   0          42m
   ```
3. Check the `kafka-controller-manager-0` pod logs.
   ```
   $ kubectl logs kafka-controller-manager-0 -n knative-sources
   2019/03/19 22:25:54 Registering Components.
   2019/03/19 22:25:54 Setting up Controller.
   2019/03/19 22:25:54 Adding the Apache Kafka Source controller.
   2019/03/19 22:25:54 Starting Apache Kafka controller.
   ```

#### Apache Kafka Topic (Optional)

1. If using Strimzi, you can can set a topic modifying
   `contrib/kafka/samples/kafka-topic.yaml` with your desired:

- Topic
- Cluster Name
- Partitions
- Replicas

  ```yaml
  apiVersion: kafka.strimzi.io/v1alpha1
  kind: KafkaTopic
  metadata:
    name: knative-demo-topic
    namespace: kafka
    labels:
      strimzi.io/cluster: my-cluster
  spec:
    partitions: 3
    replicas: 1
    config:
      retention.ms: 7200000
      segment.bytes: 1073741824
  ```

2. Deploy the `KafkaTopic`

   ```shell
   $ kubectl apply -f contrib/kafka/samples/strimzi-topic.yaml
   kafkatopic.kafka.strimzi.io/knative-demo-topic created
   ```

3. Ensure the `KafkaTopic` is running.

   ```shell
   $ kubectl -n kafka get kafkatopics.kafka.strimzi.io
   NAME                 AGE
   knative-demo-topic   16s
   ```

#### Event Display

1. Build and deploy the Event Display Service.
   ```
   $ ko apply -f contrib/kafka/samples/event-display.yaml
   ...
   service.serving.knative.dev/event-display created
   ```
2. Ensure that the Service pod is running. The pod name will be prefixed with
   `event-display`.
   ```
   $ kubectl get pods
   NAME                                            READY     STATUS    RESTARTS   AGE
   event-display-00001-deployment-5d5df6c7-gv2j4   2/2       Running   0          72s
   ...
   ```

#### Apache Kafka Event Source

1. Modify `contrib/kafka/samples/event-source.yaml` accordingly with bootstrap
   servers, topics, etc...

   NOTE: If using an internal Apache Kafka cluster, you may need to ensure
   you've specified the correct variables set in any `KafkaTopic` and
   `event-source`. For example, the following source could be used:

   ```yaml
   apiVersion: sources.eventing.knative.dev/v1alpha1
   kind: KafkaSource
   metadata:
     name: kafka-source
   spec:
     consumerGroup: knative-group
     bootstrapServers: my-cluster-kafka-bootstrap.kafka:9092 #note the kafka namespace
     topics: knative-demo-topic
     sink:
       apiVersion: serving.knative.dev/v1alpha1
       kind: Service
       name: event-display
   ```

2. Build and deploy the event source.
   ```
   $ ko apply -f contrib/kafka/samples/event-source.yaml
   ...
   kafkasource.sources.eventing.knative.dev/kafka-source created
   ```
3. Check that the event source pod is running. The pod name will be prefixed
   with `kafka-source`.
   ```
   $ kubectl get pods
   NAME                                  READY     STATUS    RESTARTS   AGE
   kafka-source-xlnhq-5544766765-dnl5s   1/1       Running   0          40m
   ```
4. Ensure the Apache Kafka Event Source started with the necessary
   configuration.
   ```
   $ kubectl logs kafka-source-xlnhq-5544766765-dnl5s
   {"level":"info","ts":"2019-04-01T19:09:32.164Z","caller":"receive_adapter/main.go:97","msg":"Starting Apache Kafka Receive Adapter...","Bootstrap Server":"...","Topics":".","ConsumerGroup":"...","SinkURI":"...","TLS":false}
   ```

### Verify

1. Produce the message shown below to Apache Kafka.
   ```
   {"msg": "This is a test!"}
   ```
2. Check that the Apache Kafka Event Source consumed the message and sent it to
   its sink properly.

   ```
   $ kubectl logs kafka-source-xlnhq-5544766765-dnl5s
   ...
   {"level":"info","ts":"2019-04-15T20:37:24.702Z","caller":"receive_adapter/main.go:99","msg":"Starting Apache Kafka Receive Adapter...","bootstrap_server":"...","Topics":"knative-demo-topic","ConsumerGroup":"knative-group","SinkURI":"...","TLS":false}
   {"level":"info","ts":"2019-04-15T20:37:24.702Z","caller":"adapter/adapter.go:100","msg":"Starting with config: ","bootstrap_server":"...","Topics":"knative-demo-topic","ConsumerGroup":"knative-group","SinkURI":"...","TLS":false}
   {"level":"info","ts":1553034726.546107,"caller":"adapter/adapter.go:154","msg":"Successfully sent event to sink"}
   ```

3. Ensure the Event Display received the message sent to it by the Event Source.

   ```
   $ kubectl logs event-display-00001-deployment-5d5df6c7-gv2j4 -c user-container

   ☁️  CloudEvent: valid ✅
   Context Attributes,
     SpecVersion: 0.2
     Type: dev.knative.kafka.event
     Source: dubee
     ID: partition:0/offset:333
     Time: 2019-03-19T22:32:06.535321588Z
     ContentType: application/json
     Extensions:
       key:
   Transport Context,
     URI: /
     Host: event-display.default.svc.cluster.local
     Method: POST
   Data,
     {
       "msg": "This is a test!"
     }
   ```

## Teardown Steps

1. Remove the Apache Kafka Event Source
   ```
   $ ko delete -f contrib/kafka/samples/source.yaml
   kafkasource.sources.eventing.knative.dev "kafka-source" deleted
   ```
2. Remove the Event Display
   ```
   $ ko delete -f contrib/kafka/samples/event-display.yaml
   service.serving.knative.dev "event-display" deleted
   ```
3. Remove the Apache Kafka Event Controller
   ```
   $ ko delete -f contrib/kafka/config
   serviceaccount "kafka-controller-manager" deleted
   clusterrole.rbac.authorization.k8s.io "eventing-sources-kafka-controller" deleted
   clusterrolebinding.rbac.authorization.k8s.io "eventing-sources-kafka-controller" deleted
   customresourcedefinition.apiextensions.k8s.io "kafkasources.sources.eventing.knative.dev" deleted
   service "kafka-controller" deleted
   statefulset.apps "kafka-controller-manager" deleted
   ```
4. (Optional) Remove the Apache Kafka Topic

   ```shell
   $ kubectl delete -f contrib/kafka/samples/kafka-topic.yaml
   kafkatopic.kafka.strimzi.io "knative-demo-topic" deleted
   ```
