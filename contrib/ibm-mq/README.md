## IBM MQ source

This event source is meant to be used as a Container Source with a Knative cluster to consume messages from IBM Message Queue and send them to a Knative service/function.

### Local Usage with Docker

1. Setup local IBM MQ following [this tutorial](https://developer.ibm.com/messaging/learn-mq/mq-tutorials/mq-connect-to-queue-manager/#docker)

2. Define environment variables:
```
export PASSWORD=yourPass
```
The rest comes as default, if you change these defaults, set env variables accordingly

3. Run `docker build -t mqsource .` to build the source image from the source code

4. Run `docker run --rm -e PASSWORD=yourPass -e CONNECTION_NAME='6bc57e4c9378(1414)' --link=6bc57e4c9378 mqsource`

5. Go to `https://localhost:9443/ibmmq/console/` and add messages to the queue that you have connected to and see how they are being processed by the MQ source

```
{QueueManager:QM1 ChannelName:DEV.APP.SVRCONN ConnectionName:6bc57e4c9378(1414) UserID:app Password:admin QueueName:DEV.QUEUE.1}
Connection to QM1 succeeded.
Opened queue DEV.QUEUE.1
2019/05/07 14:31:23 New message:  &{1 0 8 -1 0 273 1208 MQSTR 0 0 [65 77 81 32 81 77 49 32 32 32 32 32 32 32 32 32 131 60 209 92 2 16 45 35] [0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] 0  QM1 mqm [0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] admin 7 IBM MQ Web Admin/REST API 20190507 14312329  [0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0] 1 0 0 -1}
```

If you want to add and remove message in bulk, build the Docker image for IBM MQ Local client from `local-client` folder and follow the instructions from the tutorial (step 5) to get messages to the MQ (enter `put 20` in the shell prompt)

### Knative usage

Edit the Container source manifest and apply it:

```
kubectl apply -f mq-source.yaml
```