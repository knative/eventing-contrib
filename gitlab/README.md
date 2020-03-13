# GitLab Event Source for Knative

GitLab Source example shows how to wire GitLab events for consumption by a
Knative Service.

## Deploy the GitLab source controller

We recommend to use [ko](https://github.com/google/ko) tool to deploy GitLab
source:

```shell
ko apply -f gitlab/config/
```

Check that the manager is running:

```shell
kubectl -n knative-sources get pods gitlab-controller-manager-0
```

With the controller running you can now move on to a user persona and setup a
GitLab webhook as well as a function that will consume GitLab events.

## Using the GitLab Event Source

You are now ready to use the Event Source and trigger functions based on GitLab
projects events.

We will:

- Create a Knative service which will receive the events. To keep things simple
  this service will simply dump the events to `stdout`, this is the so-called:
  _event_display_
- Create a GitLab access token and a random secret token used to secure the
  webhooks
- Create the event source by posting a GitLab source object manifest to
  Kubernetes

### Create a Knative Service

Create a simple Knative `service` that dumps incoming messages to its log. The
[event-display.yaml](samples/event-display.yaml) file defines this basic service
which will receive the configured GitLab event from the GitLabSource object.

```shell
kubectl -n default apply -f samples/event-display.yaml
```

### Create GitLab Tokens

1. Create a
   [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
   which the GitLab source will use to register webhooks with the GitLab API.
   The token must have an "api" access scope in order to create repository
   webhooks. Also decide on a secret token that your code will use to
   authenticate the incoming webhooks from GitLab.

1. Update a secret defined in [secret.yaml](samples/secret.yaml):

   `accessToken` is the personal access token created in step 1 and
   `secretToken` is any token of your choosing.

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
   [gitlabsource.yaml](samples/gitlabsource.yaml) with your GitLab username and
   project name. For example, if your repo URL is
   `https://gitlab.com/knative-examples/functions` then use it as the value for
   `projectUrl`.

1. Apply the yaml file using `kubectl`:

   ```shell
   kubectl -n default apply -f samples/gitlabsource.yaml
   ```

### Verify

Verify the GitLab webhook was created by looking at the list of webhooks under
**Settings >> Integrations** in your GitLab project. A hook should be listed
that points to your Knative cluster.

Create a push event and check the logs of the Pod backing the
`gitlab-event-display` knative service. You will see the GitLab event:

```
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  specversion: 0.3
  type: dev.knative.sources.gitlabsource.Push Hook
  source: https://gitlab.com/<user>/<project>
  id: f83c080f-c2af-48ff-8d8b-fd5b21c5938e
  time: 2020-03-12T11:08:41.414572482Z
  datacontenttype: application/json
Data,
  {
   <Event payload>
  }
```
