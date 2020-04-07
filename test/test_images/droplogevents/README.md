## Drop log events

These images are designed to test brokers redelivery feature.

The [receiver](receiver.go) counts each event and it doesn't accept events
based on counter value and a given `Skip` logic.

Accepted events are logged to the standard output.

### Images

- [Fibonacci](fibonacci.yaml)
- [First `NUMBER` events](first.yaml)
