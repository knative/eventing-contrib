# Kafka - Source
The Kafka Event source enables Knative Eventing integration with Kafka. When an message is produced to Kafka, the
Kafka Event Source will consume the produced message and post that message the corresponding event sink.
  
This sample demonstrates how to configure, deploy, and use the Kafka Event Source with a Knative Service.

## Build and Deploy Steps
### Prerequisites
1. An existing instance of Kafka must be running to use the Kafka Event Source.
    - In order to consume and produce messages, a topic must be created on the Kafka instance.
    - A list of brokers corresponding to Kafka instance must be obtained.
2. Install the `ko` CLI for building and deploying purposes.
    ```
    go get github.com/google/go-containerregistry/cmd/ko
    ```
3. A container registry, such as a Docker Hub account, is required.
    - Export the `KO_DOCKER_REPO` environment variable with a value denoting the container registry to use.
      ```
      export KO_DOCKER_REPO="docker.io/YOUR_REPO"
      ```

### Build and Deployment
The following steps build and deploy the Kafka Event Controller, Source, and an Event Display Service.
- Assuming current working directory is the project root `eventing-sources`.

#### Kafka Event Controller
1. Build the Kafka Event Controller and configure a Service Account, Cluster Role, Controller, and Source.
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
3. Check the `controller-manager-0` pod logs.
    ```
    $ kubectl logs kafka-controller-manager-0 -n knative-sources
    2019/03/19 22:25:54 Registering Components.
    2019/03/19 22:25:54 Setting up Controller.
    2019/03/19 22:25:54 Adding the Kafka Source controller.
    2019/03/19 22:25:54 Starting Kafka controller.
    ```

#### Kafka Event Source
1. Modify `contrib/kafka/samples/event-source.yaml` accordingly with brokers, topic, etc... 
2. Build and deploy the event source.
    ```
    $ ko apply -f contrib/kafka/samples/event-source.yaml
    ...
    kafkasource.sources.eventing.knative.dev/kafka-source created
    ```
3. Check that the event source pod is running. The pod name will be prefixed with `kafka-source`.
    ```
    $ kubectl get pods
    NAME                                  READY     STATUS    RESTARTS   AGE
    kafka-source-xlnhq-5544766765-dnl5s   1/1       Running   0          40m
    ```
4.  Ensure the Kafka Event Source started with the necessary configuration.
    ```
    $ kubectl logs kafka-source-xlnhq-5544766765-dnl5s
    {"level":"info","ts":"2019-03-19T22:31:52.689Z","caller":"receive_adapter/main.go:97","msg":"Starting Kafka Receive Adapter...","adapter":{"Brokers":"...","Topic":"...","ConsumerGroup":"...","Net":{"SASL":{"Enable":true,"User":"...","Password":"..."},"TLS":{"Enable":true}},"SinkURI":"http://event-display.default.svc.cluster.local/"}}
    ```

#### Event Display 
1. Build and deploy the Event Display Service.
    ```
    $ ko apply -f contrib/kafka/samples/event-display.yaml
    ...
    service.serving.knative.dev/event-display created
    ``` 
2. Ensure that the Service pod is running. The pod name will be prefixed with `event-display`.
    ```
    $ kubectl get pods
    NAME                                            READY     STATUS    RESTARTS   AGE
    event-display-00001-deployment-5d5df6c7-gv2j4   2/2       Running   0          72s
    ...
    ```

### Verify
1. Produce the message shown below to Kafka.
    ```
    {"msg": "This is a test!"}
    ```
2. Check that the Kafka Event Source consumed the message and sent it to its sink properly.
    ```
    $ kubectl logs kafka-source-xlnhq-5544766765-dnl5s
    ...
    {"level":"info","ts":1553034726.5351956,"logger":"fallback","caller":"adapter/adapter.go:121","msg":"Received: {value 15 0 {\"msg\": \"This is a test!\"} <nil>}"}
    {"level":"info","ts":1553034726.546107,"logger":"fallback","caller":"adapter/adapter.go:154","msg":"Successfully sent event to sink"}
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
1. Remove the Kafka Event Source
    ```
    $ ko delete -f contrib/kafka/samples/source.yaml
    kafkasource.sources.eventing.knative.dev "kafka-source" deleted
    ```
2. Remove the Event Display
    ```
    $ ko delete -f contrib/kafka/samples/event-display.yaml
    service.serving.knative.dev "event-display" deleted
    ```
3. Remove the Kafka Event Controller
    ```
    $ ko delete -f contrib/kafka/config
    serviceaccount "kafka-controller-manager" deleted
    clusterrole.rbac.authorization.k8s.io "eventing-sources-kafka-controller" deleted
    clusterrolebinding.rbac.authorization.k8s.io "eventing-sources-kafka-controller" deleted
    customresourcedefinition.apiextensions.k8s.io "kafkasources.sources.eventing.knative.dev" deleted
    service "kafka-controller" deleted
    statefulset.apps "kafka-controller-manager" deleted
    ```
