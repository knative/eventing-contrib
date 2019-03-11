FROM centos:7
MAINTAINER Simon Woodman <swoodman@redhat.com>
ARG BINARY=./kafkaeventsource

COPY ${BINARY} /opt/kafkaeventsource
ENTRYPOINT ["/opt/kafkaeventsource"]