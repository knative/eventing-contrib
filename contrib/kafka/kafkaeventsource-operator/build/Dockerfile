FROM alpine:3.6

USER nobody

ADD build/_output/bin/kafkaeventsource-operator /usr/local/bin/kafkaeventsource-operator

ENTRYPOINT ["/usr/local/bin/kafkaeventsource-operator"]