# End to end tests

- [Running e2e tests](../README.md#running-e2e-tests)

## Adding end to end tests

Knative Eventing Sources e2e tests test the end to end functionality of the
[Knative Eventing Sources](#requirements) to verify they work as expected.

### Requirements

The e2e tests are used to test whether events flow from Sources to Sinks as
designed.

The e2e tests **MUST**:

1. Provide frequent output describing what actions they are undertaking,
   especially before performing long running operations.
1. Follow Golang best practices.
