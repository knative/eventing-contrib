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
