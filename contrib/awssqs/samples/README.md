# AWS SQS - Source

These samples demonstrate how to configure an AWS SQS source, consuming events
from an AWS SQS queue.

## Deployment Steps

The steps below assume that you have cloned the
[eventing-sources](https://github.com/knative/eventing-sources) repo and run the
commands from the root.

### Prerequisites

1.  Create an [AWS SQS queue](https://aws.amazon.com/sqs/).

1.  Setup
    [Knative Eventing](https://github.com/knative/docs/tree/master/eventing).

1.  The
    [in-memory `ClusterChannelProvisioner`](https://github.com/knative/eventing/tree/master/config/provisioners/in-memory-channel)
    should be installed in your cluster. At the time of writing (release v0.5.0)
    it is part of the default instructions so you will probably have it there
    already.

### Create a channel and subscriber

```shell
ko apply -f contrib/awssqs/samples/display-resources.yaml
```

The sample provided will configure an in-memory channel (named `awssqs-test` and
a subscription to a consumer named `awssqs-event-display`) that will simply
display incoming events to stdout.

At this point we have a channel and a subscriber ready to receive events, we
will now create an awssqs source resource that starts pulling events from a SQS
queue.

### Set up credentials

Acquire
[AWS Credentials](https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html)
for the same account. Your credentials file should look like this:

     [default]
     aws_access_key_id = ...
     aws_secret_access_key = ...

Then create a secret for the downloaded key:

```shell
kubectl -n knative-sources create secret generic awssqs-source-credentials --from-file=credentials=PATH_TO_CREDENTIALS_FILE
```

### Deployment

Deploy the `AwsSqsSource` controller as part of eventing-source's controller.

```shell
ko -n default apply -f contrib/awssqs/config/
```

Note that if the `Source` Service Account secret is in a non-default location,
you will need to update the YAML first.

Replace the place holders in `samples/awssqs-source.yaml`.

- `QUEUE_URL` should be replaced with your AWS SQS queue URL.

  ```shell
  export QUEUE_URL=https://sqs-eu-west-1.amazonaws.com/1234234234/my-queue
  sed -i "s|QUEUE_URL|$QUEUE_URL|" awssqs-source.yaml
  ```

Now deploy `awssqs-source.yaml`.

```shell
ko apply -f contrib/awssqs/samples/awssqs-source.yaml
```

You can use [kail](https://github.com/boz/kail/) to tail the logs of the
subscriber.

```shell
kail -d awssqs-event-display --since=10m
```

### Publish messages to the queue

Publish messages to your AWS SQS queue.

```shell
aws sqs send-message --queue-url $QUEUE_URL --message-body "Hello World!"
```

Where the `QUEUE_URL` variable contains the full AWS SQS URL (e.g.
`https://sqs.us-east-1.amazonaws.com/80398EXAMPLE/MyQueue`)

### Verify

The window with subscriber logs (in the window where you executed
`kail -d awssqs-event-display --since=10m` before) should now be displaying log
lines similar to:)

```
{"ID":"284375451531353","Data":"Hello World!","Attributes":null,"PublishTime":"2018-10-31T00:00:00.00Z"}

```
