# Kubernetes API Server Source

These samples show how to configure a Kubernetes API server source.

## Prerequisites

1. Install and configure [ko](https://www.knative.dev/v0.3-docs/concepts/resources/)

1. Install the [API server source from this directory](../config/) from source:

   ```shell
   ko apply --filename https://github.com/knative/eventing-sources/tree/master/contrib/apiserver/config/
   ```

   Or install a release version (TODO: link to released component).

1. Install the sample in the default namespace:

   ```shell
   ko apply -f .
   ```

### Verify

Verify that an event is sent when a config map is created.

1. Create a config map.

   ```shell
   kubectl create configmap special-config --from-literal=special.how=very --from-literal=special.type=charm
   ```

1. Use [`kail`](https://github.com/boz/kail) to tail the logs of the subscriber.

   ```shell
   kail -d event-display --since=10m
   ```

You should see log lines similar to:

```
☁️  CloudEvent: valid ✅
Context Attributes,
  SpecVersion: 0.2
  Type: dev.knative.apiserver.object.update
  Source: /api/v1/namespaces/default/configmaps/special-config
  ID: 870a7e9e-5c97-11e9-b27f-82c318d3f413
  Time: 2019-04-11T20:22:28Z
  ContentType: application/json
Transport Context,
  URI: /
  Host: event-display.default.svc.cluster.local
  Method: POST
Data,
  {
    "kind": "ConfigMap",
    "namespace": "default",
    "name": "special-config",
    "apiVersion": "v1"
  }
```
