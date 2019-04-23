# Heartbeats Sample

This is a sample showing ContainerSource. The sample is for developers of
sources to see a working example.

## Deployment Steps

### Prerequisites

- [ ] Install [Knative](https://www.knative.dev/docs/install/)
- [ ] Install
      [ko](https://github.com/google/go-containerregistry/tree/master/cmd/ko)

### Deploy Heartbeats Sender and Receiver

Create a ContainerSource (sender) and a Service (receiver):

```bash
ko apply -f ./samples/heartbeats/
```

## Verify

View sender logs:

```shell
kubectl logs --tail=50 -l source=heartbeats-sender -c source
```

View receiver logs:

```shell
kubectl logs --tail=50 -l serving.knative.dev/service=heartbeats-receiver -c user-container
```

## Uninstall

```bash
ko delete -f ./samples/heartbeats/
```
