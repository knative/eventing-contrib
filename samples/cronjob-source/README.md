# CronJob - Source

## Deployment Steps

### Prerequisites

1. Setup
   [Knative Eventing](https://github.com/knative/docs/tree/master/eventing).
1. Install the
   [in-memory `ClusterChannelProvisioner`](https://github.com/knative/eventing/tree/master/config/provisioners/in-memory-channel).
   - Note that you can skip this if you choose to use a different type of
     `Channel`. If so, you will need to modify `channel.yaml` before deploying
     it.
1. Create a `Channel`. You can use your own `Channel` or use the provided
   sample, which creates `cj-1`. If you use your own `Channel` with a different
   name, then you will need to alter other commands later.

   ```shell
   kubectl -n default apply -f samples/cronjob-source/channel.yaml
   ```

### Deployment

1. Deploy the `CronJobSource` controller as part of eventing-source's
   controller.

   ```shell
   ko -n default apply -f config/
   ```

   - Note that if the `Source` Service Account secret is in a non-default
     location, you will need to update the YAML first.

1. Deploy `source.yaml`.

   ```shell
   kubectl -n default apply -f samples/cronjob-source/source.yaml
   ```

1. Variables in `source.yaml`.
   - `schedule` takes a [Cron](https://en.wikipedia.org/wiki/Cron) format
     string, such as `0 * * * *` or `@hourly`.
   - `data` is optional, it will be sent to downstream function as message
     "body".

### Subscriber

In order to check the `CronJobSource` is fully working, we will create a simple
Knative Service that dumps incoming messages to its log and create a
`Subscription` from the `Channel` to that Knative Service.

1. Setup [Knative Serving](https://github.com/knative/docs/tree/master/serving).
1. If the deployed `CronJobSource` is pointing at a `Channel` other than `cj-1`,
   modify `subscriber.yaml` by replacing `cj-1` with that `Channel`'s name.
1. Deploy `subscriber.yaml`.

   ```shell
   ko -n default apply -f samples/cronjob-source/subscriber.yaml
   ```

### Verify

We will verify that the message was sent into the Knative eventing system by
looking at what is downstream of the `CronJobSource`. If you deployed the
[Subscriber](#subscriber), then continue using this section. If not, then you
will need to look downstream yourself.

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d message-dumper -c user-container --since=10m
   ```

You should see log lines similar to:

```json
{
  "ID": "1543616460000180552-203",
  "EventTime": "2018-11-30T22:21:00.000186721Z",
  "Body": "{\"message\": \"Hello world!\"}"
}
```
