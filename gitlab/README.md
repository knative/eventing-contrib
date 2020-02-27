# GitLab Event Source for Knative

GitLab Source example shows how to wire GitLab events for consumption
by a Knative Service.

## Deploy the GitLab source controller

We recommend to use [ko](https://github.com/google/ko) tool to deploy GitLal source:

```shell
ko apply -f gitlab/config/
```

Check that the manager is running:

```shell
kubectl -n knative-sources get pods gitlab-controller-manager-0
```

With the controller running you can now move on to a user persona and
setup a GitLab webhook as well as a function that will consume GitLab events.

## Using the GitLab Event Source

You are now ready to use the Event Source and trigger functions based
on GitLab projects events.

We will:

* Create a Knative service which will receive the events. To keep things simple
  this service will simply dump the events to `stdout`, this is the so-called: _event_display_
* Create a GitLab access token and a random secret token used to secure the webhooks
* Create the event source by posting a GitLab source object manifest to Kubernetes

### Create a Knative Service

Create a simple Knative `service` that dumps incoming messages to its log.
The [event-display.yaml](samples/event-display.yaml) file defines this basic
service which will receive the configured GitLab event from the GitLabSource object.
The contents of the [event-display.yaml](samples/event-display.yaml)
file are as follows:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: event-display
spec:
  template:
    spec:
      containers:
      - image: gcr.io/knative-releases/github.com/knative/eventing-sources/cmd/event_display
```

Enter the following command to create the service from [event-display.yaml](samples/event-display.yaml):

```shell
kubectl -n default apply -f samples/event-display.yaml
```

### Create GitLab Tokens

1. Create a [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
  which the GitLab source will use to register webhooks with the GitLab API.
  The token must have an "api" access scope in order to create repository webhooks.
  Also decide on a secret token that your code will use to authenticate the
  incoming webhooks from GitLab.

1. Update a secret defined in [secret.yaml](samples/secret.yaml):

  ```yaml
  apiVersion: v1
  kind: Secret
  metadata:
    name: gitlabsecret
  type: Opaque
  stringData:
    accessToken: <personal_access_token_value>
    secretToken: <random_string>
  ```

  Where `accessToken` is the personal access token created in step 1.
  and `secretToken` (`random_string` above) is any token of your choosing.

  Hint: you can generate a random _secretToken_ with:

  ```shell
  head -c 8 /dev/urandom | base64
  ```

1. Apply the gitlabsecret using `kubectl`.

  ```shell
  kubectl -n default apply -f samples/secret.yaml
  ```

### Create Event Source for GitLab Events

1. In order to receive GitLab events, you have to create a concrete Event Source
  for a specific namespace. Replace the `projectUrl` value in the file
  `gitlabeventbinding.yaml` with your GitLab username and project name. For example,
  if your repo URL is `https://gitlab.com/knative-examples/functions`
  then use it as the value for `projectUrl`.

  ```yaml
  apiVersion: source.knative.dev/v1alpha1
  kind: GitLabSource
  metadata:
    name: gitlabsource-sample
  spec:
    eventTypes:
    - push_events
    - issues_events
    projectUrl: "<project url>"
    accessToken:
      secretKeyRef:
        name: gitlabsecret
        key: accessToken
    secretToken:
      secretKeyRef:
        name: gitlabsecret
        key: secretToken
    sink:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: gitlab-event-display
  ```

1. Apply the yaml file using `kubectl`:

  ```shell
  kubectl -n default apply -f samples/gitlabsource.yaml
  ```

### Verify

Verify the GitLab webhook was created by looking at the list of
webhooks under **Settings >> Integrations** in your GitLab project. A hook
should be listed that points to your Knative cluster.

Create a push event and check the logs of the Pod backing the
`event-display`. You will see the GitLab event.
