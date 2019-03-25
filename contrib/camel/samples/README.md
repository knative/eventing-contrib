# Camel Source

These samples show how to configure a Camel source. It is a event source that
can leverage one of the
[250+ Apache Camel components](https://github.com/apache/camel/tree/master/components)
for generating events.

## Deployment Steps

These steps assume that you have checked out the repo and have a shell in this
`samples` directory.

### Prerequisites

1. Install the [Apache Camel K](https://github.com/apache/camel-k) Operator in
   any namespace where you want to run Camel sources.

   The preferred version that is compatible with Camel sources is
   [Camel K v0.2.0](https://github.com/apache/camel-k/releases/tag/0.2.0).

   Installation instruction are provided on the
   [Apache Camel K Github repository](https://github.com/apache/camel-k#installation).
   Documentation includes specific instructions for common Kubernetes
   environments, including development clusters.

1. Install the Camel Source from the [yaml files in the config dir](../config/)
   from source:

   ```shell
   ko apply -f ../config/
   ```

   Or install a release version (TODO: link to released component).

### Create a Channel and a Subscriber

In order to check a `CamelSource` is fully working, we will create:

- a simple Knative Service that dumps incoming messages to its log
- a in-memory channel named `camel-test` that will buffer messages created by
  the event source
- a subscription to direct messages on the test channel to the event display

Deploy `display_resources.yaml`, building it from source:

```shell
ko apply -f display_resources.yaml
```

### Run a CamelSource using the Timer component

The samples directory contains some sample sources that can be used to generate
events.

The simplest one, that does not require additional configuration is the "timer"
source.

If you want, you can customize the source behavior using options available in
the Apache Camel documentation for the
[timer component](https://github.com/apache/camel/blob/master/camel-core/src/main/docs/timer-component.adoc).
All Camel components are documented in the
[Apache Camel github repository](https://github.com/apache/camel/tree/master/components).

Install the [timer CamelSource](source_timer.yaml) from source:

```shell
ko apply -f source_timer.yaml
```

We will verify that the published message was sent into the Knative eventing
system by looking at what is downstream of the `CamelSource`.

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d camel-event-display --since=10m
   ```

If you've deployed the timer source, you should see log lines appearing every 3
seconds.

### Run a CamelSource using the Telegram component

Another useful component available with Camel is the Telegram component. It can
be used to forward messages of a [Telegram](https://telegram.org/) chat into
Knative channels as events.

Before using the provided Telegram CamelSource example, you need to follow the
instructions on the Telegram website for creating a
[Telegram Bot](https://core.telegram.org/bots). The quickest way to create a bot
is to contact the [Bot Father](https://telegram.me/botfather), another Telegram
Bot, using your preferred Telegram client (mobile or web). After you create the
bot, you'll receive an **authorization token** that is needed for the source to
work.

First, edit the [telegram CamelSource](source_telegram.yaml) and put the
authorization token, replacing the `<put-your-token-here>` placeholder.

To reduce noise in the event display, you can remove the previously created
timer CamelSource from the namespace:

```shell
kubectl delete camelsource camel-timer-source
```

Install the [telegram CamelSource](source_telegram.yaml) from source:

```shell
ko apply -f source_telegram.yaml
```

Start again kail and keep it open on the event display:

```shell
kail -d camel-event-display --since=10m
```

Now, you can contact your bot with any Telegram client. Each message you'll send
to the bot will be printed by the event display as cloudevent.
