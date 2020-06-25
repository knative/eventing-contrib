# Conformance tests

Conformance tests verifies kantive eventing implementation for expected behavior
described in
[specification](https://github.com/knative/eventing/tree/master/docs/spec).

## Running conformance tests

Run test with e2e tag and optionally select conformance test

> NOTE: Make sure you have built the
> [test images](https://github.com/knative/eventing/tree/master/test#building-the-test-images)!

```bash
go test -v -tags=e2e -count=1 ./test/conformance/...

go test -v -timeout 30s -tags e2e knative.dev/eventing/test/conformance -run ^TestChannelStatus$ -channels=messaging.knative.dev/v1alpha1:NatssChannel,messaging.knative.dev/v1alpha1:KafkaChannel
```
