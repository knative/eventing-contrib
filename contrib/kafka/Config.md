# EventSource Configuration options

Internally the KafkaEventSource uses the [Sarama Cluster library](https://github.com/bsm/sarama-cluster).
Most of the options listed here are passed directly through to the [Sarama Config](https://godoc.org/github.com/Shopify/sarama#Config).
See the underlying docs for further information.

## Required

* bootstrap (string): Comma separated list of hostname:port pairs for the Kafka Broker. See [Advanced](#advanced) options if the Apache Kafka brokers are external to the Kubernetes cluster.

* topic (string): Name of the Kafka topic to consume messages from.

## Optional

* replicas (int): Number of replica consumer pods to create. If > 1 consider setting `consumerGroupId`. (default 1)

* consumerGroupID (string): Name of the ConsumerGroup to use when connecting to the Kafka topic. Each message in the topic will be delivered to exactly one of the consumers in each ConsumerGroup. (default empty which generates a new id per consumer)

* net.maxOpenRequests (int): How many outstanding requests a connection is allowed to have before sending on it blocks (default 5)

* net.keepAlive (int): The keep alive period for the active connection. Zero disables keep alives. (default 0)

* net.sasl.enable (bool): Whether or not to use SASL authentication when connecting to the broker (default false)

* net.sasl.handshake (bool): Whether or not to send the Kafka SASL handshake first if enabled. You should only set this to false if you're using a non-Kafka SASL proxy. (default true)

* net.sasl.user (string): SASL username

* net.sasl.password (string): SASL password

* consumer.maxWaitTime (int64): The maximum amount of time the broker will wait for Consumer.Fetch.Min bytes to become available before it returns fewer than that anyways. The default is 250ms, since 0 causes the consumer to spin when no events are available. 100-500ms is a reasonable range for most cases. (default 250000000)

* consumer.maxProcessingTime (int64): The maximum amount of time the consumer expects a message takes to process for the user. If writing to the Messages channel takes longer than this, that partition will stop fetching more messages until it can proceed again (defalt 100000000)

* consumer.offsets.initial (string): Defines whether the consumer will receive all historical messsages which are retained in the topic or only messages the broker receives after the client connects. Should be set to either `OffsetNewest` (only new messages) or `OffsetOldest` (receive all messages the broker holds). (Default `OffsetNewest`)

* consumer.offsets.commitInterval (int64): How frequently to commit updated offsets. (default 1000000000)

* consumer.offsets.retention (int): The retention duration for committed offsets. If zero, disabled in which case the `offsets.retention.minutes` option on the broker will be used. (default 0 therefore disabled)

* consumer.offsets.retry.max (int): The total number of times to retry failing commit requests during OffsetManager shutdown (default 3)

* channelBufferSize (int): The number of events to buffer in internal and external channels. This permits the consumer to continue processing some messages in the background while user code is working, greatly improving throughput. (default  256)

* group.partitionStrategy (string): The strategy to use for the allocation of partitions to consumers. Should be either `RoundRobin` or `Range`. (default  range)

* group.session.timeout (int64): The allowed session timeout for registered consumers. (default 30000000000)

## Advanced

* externalIPRanges (string): This is required to allow the Kafka EventSource to connect to Apache Kafka brokers which are external to the kubernetes cluster. Istio is used internally by Knative to route traffic and this setting configures a set of IP CIDRs which are excluded from Istio routing. Internally this sets the kubernetes label `traffic.sidecar.istio.io/excludeOutboundIPRanges` on the pods. Avoid setting this to `0.0.0.0/0` as this or other large blocks as the EventSource may not be able to reach the channel. Multiple blocks can be comma separated.

## Example

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: KafkaEventSource
metadata:
  name: example-kafkaeventsource
spec:
  bootstrap: example-broker.com:9092
  topic: topic
  consumerGroupID: group1
  replicas: 1
  externalIPRanges: 128.0.0.1/24  
  kafkaVersion: 2.0.0
  net:
    maxOpenRequests: 5
    keepAlive: 0
    sasl:
      enable: false
      handshake: false
      user: user
      password: password
  consumer:
    maxWaitTime: 250000000
    maxProcessingTime: 100000000
    offsets:  
      commitInterval: 1000000000
      initial: OffsetNewest
      retention: 0
      retry:
        max: 3
  channelBufferSize: 256
  group:
    partitionStrategy: range  
    session:
      timeout: 30000000000
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    name: testchannel
```
