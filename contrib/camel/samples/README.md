# Camel Source

These samples show how to configure a Camel source. It is a event source that
can leverage one of the [250+ Apache Camel components](https://github.com/apache/camel/tree/master/components)
for generating events.

## Deployment Steps

These steps assume that you have checked out the repo and have a shell in this
`samples` directory.

### Prerequisites

1. Install the
   [Camel Source from this yaml](../config/default-camel.yaml) from
   source:

   ```shell
   ko apply --filename https://github.com/knative/eventing-sources/tree/master/contrib/camel/config/default-camel.yaml
   ```

   Or install a release version (TODO: link to released component).

### Create a Channel and a Subscriber

In order to check a `CamelSource` is fully working, we will create:
- a simple Knative Service that dumps incoming messages to its log
- a in-memory channel named `camel-test` that will buffer messages created by the event source
- a subscription to direct messages on the test channel to the dumper service

Deploy `dumper_resources.yaml`, building it from source:

```shell
ko apply -f dumper_resources.yaml
```

### Run a CamelSource

The samples directory contains some sample sources that can be used to generate events.

The simplest one, that does not require additional configuration is the "timer" source.

If you want, you can customize the source behavior using options available in the Apache Camel documentation for the
[timer component](https://github.com/apache/camel/blob/master/camel-core/src/main/docs/timer-component.adoc).
All Camel components are documented in the [Apache Camel github repository](https://github.com/apache/camel/tree/master/components).

Install the [timer CamelSource](source_timer.yaml) from source:

   ```shell
   ko apply --filename https://github.com/knative/eventing-sources/tree/master/contrib/camel/samples/source_timer.yaml
   ```

### Verify

We will verify that the published message was sent into the Knative eventing
system by looking at what is downstream of the `CamelSource`.

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d camel-message-dumper --since=10m
   ```

If you've deployed the timer source, you should see log lines appearing every 3 seconds.

