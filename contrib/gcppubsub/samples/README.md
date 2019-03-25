# GCP Cloud Pub/Sub - Source

[See the Knative documentation for a full example of this source.](https://github.com/knative/docs/tree/master/docs/eventing/samples/gcp-pubsub-source)

This sample shows how to configure the GCP PubSub event source. This event
source is most useful as a bridge from other GCP services, such as
[Cloud Storage](https://cloud.google.com/storage/docs/pubsub-notifications),
[IoT Core](https://cloud.google.com/iot/docs/how-tos/devices) and
[Cloud Scheduler](https://cloud.google.com/scheduler/docs/creating#).

## Deployment Steps

These steps assume that you have checked out the repo and have a shell in this
`samples` directory.

### Prerequisites

1. Create a
   [Google Cloud Project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
1. Enable the 'Cloud Pub/Sub API' on that project.

   ```shell
   gcloud services enable pubsub.googleapis.com
   ```

1. Install the [GCP PubSub Source from this directory](../config/) from source:

   ```shell
   ko apply --filename https://github.com/knative/eventing-sources/tree/master/contrib/gcppubsub/config/
   ```

   Or install a release version (TODO: link to released component).

1. Create a GCP
   [Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
   It is simplest to create a single account for both registering subscriptions
   and reading messages, but it is possible to separate the accounts for the
   controller (which creates PubSub subscriptions) and the receive adapter
   (which reads the messages from the subscription). This is left as an exercise
   for the reader.

   1. Create a new service account named `knative-source` with the following
      command:

      ```shell
      gcloud iam service-accounts create knative-source
      ```

   1. Give that Service Account the 'Pub/Sub Editor' role on your GCP project:

      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:knative-source@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/pubsub.editor
      ```

   1. Download a new JSON private key for that Service Account. **Be sure not to
      check this key into source control!**

      ```shell
      gcloud iam service-accounts keys create knative-source.json \
        --iam-account=knative-source@$PROJECT_ID.iam.gserviceaccount.com
      ```

   1. Create two secrets on the kubernetes cluster with the downloaded key:

      ```shell
      # Note that the first secret may already have been created when installing
      # Knative Eventing. The following command will overwrite it. If you don't
      # want to overwrite it, then skip this command.
      kubectl -n knative-sources create secret generic gcppubsub-source-key --from-file=key.json=knative-source.json --dry-run -o yaml | kubectl apply --filename -

      # The second secret should not already exist, so just try to create it.
      kubectl -n default create secret generic google-cloud-key --from-file=key.json=knative-source.json
      ```

      `gcppubsub-source-key` and `key.json` are pre-configured values in the
      `gcppubsub-controller` StatefulSet which manages your Eventing sources.

      `google-cloud-key` and `key.json` are pre-configured values in
      [`gcp-pubsub-source.yaml`](./gcp-pubsub-source.yaml). If you choose to
      create a second account, this account only needs
      `roles/pubsub.Subscriber`.

### Deployment

1. Decide on a topic name, and export variables for your GCP project ID and
   topic name in your shell:

   ```shell
   export PROJECT_ID=$(gcloud config get-value project)
   export TOPIC_NAME=laconia
   ```

1. Create a GCP PubSub Topic. Set `$TOPIC_NAME` to the name of the topic you
   want to receive on. This will be used in subsequent steps as well.

   ```shell
   gcloud pubsub topics create $TOPIC_NAME
   ```

1. Replace the place holders in `gcp-pubsub-source.yaml`.

   - `MY_GCP_PROJECT` should be replaced with your
     [Google Cloud Project](https://cloud.google.com/resource-manager/docs/creating-managing-projects)'s
     ID.

   - `TOPIC_NAME` should be replaced with your GCP PubSub Topic's name. It
     should be the unique portion within the project. E.g. `laconia`, not
     `projects/my-gcp-project/topics/laconia`.

   - `event-display` should be replaced with the name of the `Addressable` you
     want messages sent to. If you deployed the event display Service as the
     destination for messages, you can leave this as `event-display`.

   - `gcpCredsSecret` should be replaced if you are using a non-default secret
     or key name for the receive adapter's credentials.

   You can do this with `sed`:

   ```shell
   sed -e "s/MY_GCP_PROJECT/$PROJECT_ID/g" -e "s/TOPIC_NAME/$TOPIC_NAME/g" gcp-pubsub-source.yaml | kubectl apply -f -
   ```

### Subscriber

In order to check the `GcpPubSubSource` is fully working, we will create a
simple Knative Service that displays incoming events to its log, and direct the
PubSub source to send messages directly (i.e. without buffering or fanout within
the cluster).

Deploy `event-display.yaml`, building it from source:

```shell
ko apply -f event-display.yaml
```

### Publish

Publish messages to your GCP PubSub Topic.

```shell
gcloud pubsub topics publish $TOPIC_NAME --message="Hello World!"
```

### Verify

We will verify that the published message was sent into the Knative eventing
system by looking at what is downstream of the `GcpPubSubSource`. If you
deployed the [Subscriber](#subscriber), then continue using this section. If
not, then you will need to look downstream yourself.

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d event-display --since=10m
   ```

You should see log lines similar to:

```
{"ID":"284375451531353","Data":"SGVsbG8gV29ybGQh","Attributes":null,"PublishTime":"2018-10-31T00:00:00.00Z"}

```
