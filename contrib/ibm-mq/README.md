## IBM MQ source

This event source is meant to be used within a Knative cluster in order to consume messages from IBM Message Queue.

### Local Build and Usage

1. Setup a local IBM MQ with the following commands:

```shell
docker volume create qmdata
docker network create mqnetwork
docker run --env LICENSE=accept \
           --env MQ_QMGR_NAME=QM1 \
           --volume qmdata:/mnt/mqm \
           --publish 1414:1414 \
           --publish 9443:9443 \
           --network mqnetwork \
           --network-alias qmgr \
           --detach \
           --env MQ_APP_PASSWORD=password \
           --name mq \
           ibmcom/mq:latest
```

If you face any issues please follow the official [tutorial](https://developer.ibm.com/messaging/learn-mq/mq-tutorials/mq-connect-to-queue-manager/#docker)

1. Connect the Knative event display:

```shell
docker run --name display --net=container:mq -d gcr.io/knative-releases/github.com/knative/eventing-sources/cmd/event_display@sha256:37ace92b63fc516ad4c8331b6b3b2d84e4ab2d8ba898e387c0b6f68f0e3081c4
```

1. To build the container image from the local source code

```shell
docker build -t mqsource .
```

1. Run this _source_ locally with:

```shell
docker run --rm -e PASSWORD=password \
                --net=container:mq \
                mqsource --sink http://localhost:8080
```

1. Open the MQ console:

Using _Admin_ and _passw0rd_ as default development credentials do:

`open https://localhost:9443/ibmmq/console/` 

Add messages to `DEV.QUEUE.1` queue and check the logs of the Knative event display to see the CloudEvents being received. They will look something like

```
$ docker logs display
☁️  cloudevents.Event
Validation: valid
Context Attributes,
  cloudEventsVersion: 0.1
  eventType: message queue item
  source: ibm:mq
  eventID: 08e05aa7-c9cc-4139-a041-9623cb101613
  eventTime: 2019-06-07T13:42:10.6321831Z
  contentType: application/json
Data,
  {
    "message_descriptor": {
      "Version": 1,
      "Report": 0,
      "MsgType": 8,
      "Expiry": -1,
      "Feedback": 0,
      "Encoding": 273,
      "CodedCharSetId": 1208,
      "Format": "MQSTR",
      "Priority": 0,
      "Persistence": 0,
      "MsgId": "QU1RIFFNMSAgICAgICAgIGCQ/1wCdo8g",
      "CorrelId": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      "BackoutCount": 0,
      "ReplyToQ": "",
      "ReplyToQMgr": "QM1",
      "UserIdentifier": "mqm",
      "AccountingToken": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
      "ApplIdentityData": "admin",
      "PutApplType": 7,
      "PutApplName": "IBM MQ Web Admin/REST API",
      "PutDate": "20190611",
      "PutTime": "11444037",
      "ApplOriginData": "",
      "GroupId": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      "MsgSeqNumber": 1,
      "Offset": 0,
      "MsgFlags": 0,
      "OriginalLength": -1
    },
    "message_data": "Hello Knative"
  }
```

### Knative usage

Edit the event source manifest to specify your queue and connection parameters then apply it like so:

```
kubectl apply -f mqsource.yaml
```

Or deploy it with `ko` (from repository root):

```
CGO_ENABLED=1 ko apply -f ./contrib/ibm-mq/
```

Please note that client requires CGO module to use IBM MQ library bindings 