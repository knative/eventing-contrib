# Knative KafkaEventSource

This deploys an Operator that can instantiate Knative Event Sources which subscribe to a Kafka Topic.

## EventSource

The image that receives messages from Kafka (Receive Adaptor) is available at `docker.io/sjwoodman/kafkaeventsource`.
To build it from source edit the `eventsource/Makefile` with suitable tags for your DockerHub organisation and perform the following commands.

```bash
make docker_build docker_push_latest
```

You will need to edit the _Operator_ with the location of the newly built image.

```bash
sed -i 's|sjwoodman|DH_ORG|g' kafkaeventsource-operator/deploy/operator.yaml
sed -i 's|sjwoodman|DH_ORG|g' operator/pkg/controller/kafkaeventsource/kafkaeventsource_controller.go
```

## Usage

The sample connects to a [Strimzi](http://strimzi.io/quickstarts/okd/) Kafka Broker running inside OpenShift.

1. Setup [Knative Eventing](https://github.com/knative/docs/tree/master/eventing).

1. Install the [in-memory `ClusterChannelProvisioner`](https://github.com/knative/eventing/tree/master/config/provisioners/in-memory-channel).
    - Note that you can skip this if you choose to use a different type of `Channel`. If so, you will need to modify `01-channel.yaml` before deploying it.

1. Install the KafkaEventSource CRD

    ```bash
    kubectl create -f kafkaeventsource-operator/deploy/crds/sources_v1alpha1_kafkaeventsource_crd.yaml
    ```

1. Setup RBAC and deploy the KafkaEventSource-operator:

    ```bash
    kubectl create -f kafkaeventsource-operator/deploy/service_account.yaml
    kubectl create -f kafkaeventsource-operator/deploy/role.yaml
    kubectl create -f kafkaeventsource-operator/deploy/role_binding.yaml
    kubectl create -f kafkaeventsource-operator/deploy/operator.yaml
    ```

1. Verify that the KafkaEventSource-operator is up and running:

    ```bash
    $ kubectl get deployment
    NAME                              DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
    kafkaeventsource-operator         1         1         1            1           2m
    ```

1. (Optional) Create a Topic in Strimzi to back the Channel.
    If you don't do this, defaults will be used for the number of partitions etc.

    ```bash
    kubectl create -f kafkaeventsource-operator/sample/00-topic.yaml
    ```

1. Create an instanace of a KafkaEventSource and wire it up to a function. This will create the necessary `Channel` and `Subscription` objects. 

    ```bash
    kubectl create -f kafkaeventsource-operator/sample/01-channel.yaml -n myproject
    kubectl create -f kafkaeventsource-operator/sample/02-eventsource.yaml -n myproject
    kubectl create -f kafkaeventsource-operator/sample/03-service.yaml -n myproject
    kubectl create -f kafkaeventsource-operator/sample/04-subscription.yaml -n myproject
    ```

1. Verify that the EventSource has been started (note that your pod suffix will be different).

    ```bash
    $ kubectl get pods
    NAME                                          READY     STATUS    RESTARTS   AGE
    example-kafkaeventsource-6b6477f95d-tc4nd     2/2       Running   1          2m
    ```

1. Send some Kafka messages

    ```bash
    $ oc exec -it my-cluster-kafka-0 -- bin/kafka-console-producer.sh --broker-list localhost:9092 --topic testtopic
    > test message
    ```

## Configuration

A description of the available options for configuring the EventSource is available [here](Config.md)

## Message Keys

A String representation of the `Key` from the Kafka message is propogated in the `CE-X-Kafka-Key` CloudEvent header.
If you are consuming the CloudEvents using an SDK this value should be present in the `Extensions` structure.

## Building EventSource Operator

The KafkaEventSource Operator is built using the [Operator SDK](https://github.com/operator-framework/operator-sdk).
To build the Operator image use the following command.
Note that you will need to change the name of the image created in the `deploy/operator.yaml` to match you image name.

```bash
operator-sdk build DH_ORG/IMG_NAME:latest
#eg.
operator-sdk build docker.io/sjwoodman/kafkaeventsource-operator:latest
```