/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"flag"
	"log"
	"sync"

	"github.com/caarlos0/env"
	"github.com/ibm-messaging/mq-golang/ibmmq"
	"knative.dev/pkg/cloudevents"
)

var sink string
var wg sync.WaitGroup

type config struct {
	QueueManager   string `env:"QUEUE_MANAGER" envDefault:"QM1"`
	ChannelName    string `env:"CHANNEL_NAME" envDefault:"DEV.APP.SVRCONN"`
	ConnectionName string `env:"CONNECTION_NAME" envDefault:"localhost(1414)"`
	UserID         string `env:"USER_ID" envDefault:"app"`
	Password       string `env:"PASSWORD" envDefault:"password"`
	QueueName      string `env:"QUEUE_NAME" envDefault:"DEV.QUEUE.1"`
}

//MQMessage wraps message descriptor and message body into one struct
type MQMessage struct {
	Message     *ibmmq.MQMD `json:"message_descriptor"`
	MessageData string      `json:"message_data"`
}

func init() {
	flag.StringVar(&sink, "sink", "http://localhost:8080", "where to sink events to")
}

func main() {
	flag.Parse()

	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	cloudEventsClient := cloudevents.NewClient(
		sink,
		cloudevents.Builder{
			Source:    "ibm:mq",
			EventType: "message queue item",
		},
	)

	// create IBM MQ channel definition
	channelDefinition := ibmmq.NewMQCD()
	channelDefinition.ChannelName = cfg.ChannelName
	channelDefinition.ConnectionName = cfg.ConnectionName

	// init connection security params
	connSecParams := ibmmq.NewMQCSP()
	connSecParams.AuthenticationType = ibmmq.MQCSP_AUTH_USER_ID_AND_PWD
	connSecParams.UserId = cfg.UserID
	connSecParams.Password = cfg.Password

	// setup MQ connection params
	connOptions := ibmmq.NewMQCNO()
	connOptions.Options = ibmmq.MQCNO_CLIENT_BINDING
	// connOptions.ApplName = "Triggermesh eventing"
	connOptions.ClientConn = channelDefinition
	connOptions.SecurityParms = connSecParams

	// And now we can try to connect. Wait a short time before disconnecting.
	qMgrObject, err := ibmmq.Connx(cfg.QueueManager, connOptions)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Connection to %q succeeded.\n", cfg.QueueManager)
	defer disconnect(qMgrObject)

	// Create the Object Descriptor that allows us to give the queue name
	mqod := ibmmq.NewMQOD()

	// We have to say how we are going to use this queue. In this case, to GET
	// messages. That is done in the openOptions parameter.
	openOptions := ibmmq.MQOO_INPUT_SHARED

	// Opening a QUEUE (rather than a Topic or other object type) and give the name
	mqod.ObjectType = ibmmq.MQOT_Q
	mqod.ObjectName = cfg.QueueName

	qObject, err := qMgrObject.Open(mqod, openOptions)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Opened queue", qObject.Name)
	defer close(qObject)
	defer wg.Wait()

	for {
		// The GET requires control structures, the Message Descriptor (MQMD)
		// and Get Options (MQGMO). Create those with default values.
		msgDescriptor := ibmmq.NewMQMD()
		msgOptions := ibmmq.NewMQGMO()

		// The default options are OK, but it's always
		// a good idea to be explicit about transactional boundaries as
		// not all platforms behave the same way.
		msgOptions.Options = ibmmq.MQGMO_NO_SYNCPOINT

		// Set options to wait for a maximum of 3 seconds for any new message to arrive
		msgOptions.Options |= ibmmq.MQGMO_WAIT
		msgOptions.WaitInterval = 3 * 1000 // The WaitInterval is in milliseconds

		// Create a buffer for the message data. This one is large enough
		// for the messages put by the amqsput sample.
		buffer := make([]byte, 1024)

		// Now we can try to get the message
		_, err := qObject.Get(msgDescriptor, msgOptions, buffer)
		if err != nil {
			mqret := err.(*ibmmq.MQReturn)
			if mqret != nil && mqret.MQRC == ibmmq.MQRC_NO_MSG_AVAILABLE {
				continue
			}
			break
		}

		wg.Add(1)
		go func(md *ibmmq.MQMD, messageData []byte) {
			defer wg.Done()
			data := string(bytes.Trim(messageData, "\u0000"))
			msg := MQMessage{
				Message:     md,
				MessageData: data,
			}
			if err := sendMessage(cloudEventsClient, &msg); err != nil {
				log.Printf("Failed to send message: %v\n", err)
			}
		}(msgDescriptor, buffer)
	}
}

// Disconnect from the queue manager
func disconnect(qMgrObject ibmmq.MQQueueManager) {
	if err := qMgrObject.Disc(); err != nil {
		log.Fatal(err)
	}
}

// Close the queue if it was opened
func close(object ibmmq.MQObject) {
	if err := object.Close(0); err != nil {
		log.Fatal(err)
	}
}

func sendMessage(client *cloudevents.Client, message *MQMessage) error {
	return client.Send(message)
}
