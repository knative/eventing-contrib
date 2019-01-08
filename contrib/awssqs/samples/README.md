# AWS SQS - Source

## Deployment Steps

### Prerequisites

1.  Create an [AWS SQS queue](https://aws.amazon.com/sqs/).

1.  Setup
    [Knative Eventing](https://github.com/knative/docs/tree/master/eventing).
1.  Install the
    [in-memory `ClusterChannelProvisioner`](https://github.com/knative/eventing/tree/master/config/provisioners/in-memory-channel).
    - Note that you can skip this if you choose to use a different type of
      `Channel`. If so, you will need to modify `channel.yaml` before deploying
      it.
1.  Create a `Channel`. You can use your own `Channel` or use the provided
    sample, which creates `qux-1`. If you use your own `Channel` with a
    different name, then you will need to alter other commands later.

    ```shell
    kubectl -n default apply -f samples/awssqs_source/channel.yaml
    ```

1.  Acquire
    [AWS Credentials](https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html)
    for the same account. Your credentials file should look like this:

        [default]
        aws_access_key_id = ...
        aws_secret_access_key = ...

         1. Create a secret for the downloaded key:

             ```shell
             kubectl -n knative-sources create secret generic awssqs-source-credentials --from-file=credentials=PATH_TO_CREDENTIALS_FILE
             ```

1.  Create an AWS SQS queue. Replace `QUEUE_URL` with your desired Queue URL.

### Deployment

1. Deploy the `AwsSqsSource` controller as part of eventing-source's controller.

   ```shell
   ko -n default apply -f config/default-awssqs.yaml
   ```

   - Note that if the `Source` Service Account secret is in a non-default
     location, you will need to update the YAML first.

1. Replace the place holders in `samples/awssqs_source/awssqs-source.yaml`.

   - `QUEUE_URL` should be replaced with your AWS SQS queue URL.

     ```shell
     export QUEUE_URL=https://sqs-eu-west-1.amazonaws.com/1234234234/my-queue
     sed -i "s|QUEUE_URL|$QUEUE_URL|" awssqs-source.yaml
     ```

   - `QUEUE_NAME` will be used to name the event source, you can choose any
     value that makes sense to you, although a good choice is the last segment
     of the URL.

     ```shell
     export QUEUE_NAME="my-queue"
     sed -i "s|QUEUE_NAME|$QUEUE_NAME|" awssqs-source.yaml
     ```

   - `awsCredsSecret` should be replaced with the name of the k8s secret that
     contains the AWS credentials. Change this only if you deployed an altered
     `config/default-awssqs.yaml` source config.

   - `qux-1` should be replaced with the name of the `Channel` you want messages
     sent to. If you deployed an unaltered `channel.yaml` then you can leave it
     as `qux-1`.

1. Deploy `awssqs-source.yaml`.

   ```shell
   kubectl -n default apply -f samples/awssqs_source/awssqs-source.yaml
   ```

### Subscriber

In order to check the `AwsSqsSource` is fully working, we will create a simple
Knative Service that dumps incoming messages to its log and create a
`Subscription` from the `Channel` to that Knative Service.

1. Setup [Knative Serving](https://github.com/knative/docs/tree/master/serving).
1. If the deployed `AwsSqsSource` is pointing at a `Channel` other than `qux-1`,
   modify `subscriber.yaml` by replacing `qux-1` with that `Channel`'s name.
1. Deploy `subscriber.yaml`.

   ```shell
   ko -n default apply -f samples/awssqs_source/subscriber.yaml
   ```

### Publish

Publish messages to your AWS SQS queue.

```shell
aws sqs send-message --queue-url $QUEUE_URL --message-body "Hello World!"
```

Where the `QUEUE_URL` variable contains the full AWS SQS URL (e.g.
`https://sqs.us-east-1.amazonaws.com/80398EXAMPLE/MyQueue`)

### Verify

We will verify that the published message was sent into the Knative eventing
system by looking at what is downstream of the `AwsSqsSource`. If you deployed
the [Subscriber](#subscriber), then continue using this section. If not, then
you will need to look downstream yourself.

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d message-dumper -c user-container --since=10m
   ```

You should see log lines similar to:

```
{"ID":"284375451531353","Data":"Hello World!","Attributes":null,"PublishTime":"2018-10-31T00:00:00.00Z"}

```
