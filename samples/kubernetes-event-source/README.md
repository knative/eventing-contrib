# Kubernetes Event Source example

_This sample is deprecated. Please see the official sample at
https://github.com/knative/docs/tree/master/eventing/samples/kubernetes-event-source._

Kubernetes Event Source example shows how to wire kubernetes cluster events for
consumption by a function that has been implemented as a Knative Service.

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
   sample, which creates a channel called `testchannel`. If you use your own
   `Channel` with a different name, then you will need to alter other commands
   later.

```shell
kubectl -n default apply -f samples/kubernetes-event-source/channel.yaml
```

### Service Account

1. Create a Service Account that the _Receive Adapter_ runs as. The _Receive
   Adapter_ watches for Kubernetes events and forwards them to the Knative
   Eventing Framework.

```shell
kubectl apply -f samples/kubernetes-event-source/serviceaccount.yaml
```

### Deploy Event Sources

1. Deploy the `KubernetesEventSource` controller as part of eventing-source's
   controller. This makes kubernetes events available for subscriptions.

```shell
ko apply -f config/
```

### Create Event Source for Kubernetes Events

1. In order to receive events, you have to create a concrete Event Source for a
   specific namespace. If you are wanting to consume events from a differenet
   namespace or using a different `ServiceAccount`, you need to modify the yaml
   accordingly.

```shell
kubectl apply -f samples/kubernetes-event-source/k8s-events.yaml
```

### Subscriber

In order to check the `KubernetesEventSource` is fully working, we will create a
simple Knative Service that displays incoming eventings in its log and create a
`Subscription` from the `Channel` to that Knative Service.

1. Setup [Knative Serving](https://github.com/knative/docs/tree/master/serving).
1. If the deployed `KubernetesEventSource` is pointing at a `Channel` other than
   `testchannel`, modify `subscription.yaml` by replacing `testchannel` with
   that `Channel`'s name.
1. Deploy `subscription.yaml`.

```shell
ko apply -f samples/kubernetes-event-source/subscription.yaml
```

### Create Events

Create events by launching a pod in the default namespace.

```shell
kubectl run -i --tty busybox --image=busybox --restart=Never -- sh
```

Once the shell comes up, just exit it and kill the pod.

```shell
kubectl delete pods busybox
```

### Verify

We will verify that the Kubernetes events were sent into the Knative eventing
system by looking the subscriber's logs.

```shell
kubectl logs --tail=50 -l serving.knative.dev/service=event-display -c user-container
```

Here's an example of a logged message:

```bash

☁  CloudEvent: valid ✅
Context Attributes,
  CloudEventsVersion: 0.1
  EventType: dev.knative.k8s.event
  Source: /apis/v1/namespaces/default/pods/busybox
  EventID: dbb798ae-46a2-11e9-b199-42010a8a017e
  EventTime: 2019-03-14T21:48:09Z
  ContentType: application/json
Transport Context,
  URI: /
  Host: event-display.default.svc.cluster.local
  Method: POST
Data,
  {"metadata":{"name":"busybox.158bf18e32e236e3","namespace":"default","selfLink":"/api/v1/namespaces/default/events/busybox.158bf18e32e236e3","uid":"dbb798ae-46a2-11e9-b199-42010a8a017e","resourceVersion":"298237","creationTimestamp":"2019-03-14T21:48:09Z"},"involvedObject":{"kind":"Pod","namespace":"default","name":"busybox","uid":"dabf6cbe-46a2-11e9-b199-42010a8a017e","apiVersion":"v1","resourceVersion":"11083989","fieldPath":"spec.containers{busybox}"},"reason":"Started","message":"Started container","source":{"component":"kubelet","host":"gke-knative-auto-cluster-default-pool-23c23c4f-0drf"},"firstTimestamp":"2019-03-14T21:48:09Z","lastTimestamp":"2019-03-14T21:48:09Z","count":1,"type":"Normal","eventTime":null,"reportingComponent":"","reportingInstance":""}
```
