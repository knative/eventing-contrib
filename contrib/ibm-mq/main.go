/*
Copyright (c) 2019 TriggerMesh, Inc

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
	"flag"
	"fmt"
	"log"
	"sync"

	"github.com/caarlos0/env"
	"github.com/ibm-messaging/mq-golang/ibmmq"
	"github.com/knative/pkg/cloudevents"
)

var sink string
var wg sync.WaitGroup

type config struct {
	QueueManager   string `env:"QUEUE_MANAGER" envDefault:"QM1"`
	ChannelName    string `env:"CHANNEL_NAME" envDefault:"DEV.APP.SVRCONN"`
	ConnectionName string `env:"CONNECTION_NAME" envDefault:"localhost(1414)"`
	UserID         string `env:"USER_ID" envDefault:"app"`
	Password       string `env:"PASSWORD"`
	QueueName      string `env:"QUEUE_NAME" envDefault:"DEV.QUEUE.1"`
}

func init() {
	flag.StringVar(&sink, "sink", "", "where to sink events to")
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

	fmt.Printf("%+v\n", cfg)

	// Allocate the MQCNO and MQCD structures needed for the CONNX call.
	cno := ibmmq.NewMQCNO()
	cd := ibmmq.NewMQCD()

	// Which queue manager do we want to connect to
	qMgrName := cfg.QueueManager
	// Fill in required fields in the MQCD channel definition structure
	cd.ChannelName = cfg.ChannelName
	cd.ConnectionName = cfg.ConnectionName

	// Reference the CD structure from the CNO and indicate that we definitely want to
	// use the client connection method.
	cno.ClientConn = cd
	cno.Options = ibmmq.MQCNO_CLIENT_BINDING

	// MQ V9.1.2 allows applications to specify their own names. This is ignored
	// by older levels of the MQ libraries.
	cno.ApplName = "Golang 9.1.2 ApplName"

	// Also fill in the userid and password if the MQSAMP_USER_ID
	// environment variable is set. This is the same variable used by the C
	// sample programs such as amqsput shipped with the MQ product.
	userID := cfg.UserID

	csp := ibmmq.NewMQCSP()
	csp.AuthenticationType = ibmmq.MQCSP_AUTH_USER_ID_AND_PWD
	csp.UserId = userID
	csp.Password = cfg.Password

	// Make the CNO refer to the CSP structure so it gets used during the connection
	cno.SecurityParms = csp

	// And now we can try to connect. Wait a short time before disconnecting.
	qMgrObject, err := ibmmq.Connx(qMgrName, cno)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Connection to %s succeeded.\n", qMgrName)
	defer disc(qMgrObject)

	// Create the Object Descriptor that allows us to give the queue name
	mqod := ibmmq.NewMQOD()

	// We have to say how we are going to use this queue. In this case, to GET
	// messages. That is done in the openOptions parameter.
	openOptions := ibmmq.MQOO_INPUT_EXCLUSIVE

	// Opening a QUEUE (rather than a Topic or other object type) and give the name
	mqod.ObjectType = ibmmq.MQOT_Q
	mqod.ObjectName = cfg.QueueName

	qObject, err := qMgrObject.Open(mqod, openOptions)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Opened queue", qObject.Name)
	defer close(qObject)
	defer wg.Wait()

	for {
		// The GET requires control structures, the Message Descriptor (MQMD)
		// and Get Options (MQGMO). Create those with default values.
		getmqmd := ibmmq.NewMQMD()
		gmo := ibmmq.NewMQGMO()

		// The default options are OK, but it's always
		// a good idea to be explicit about transactional boundaries as
		// not all platforms behave the same way.
		gmo.Options = ibmmq.MQGMO_NO_SYNCPOINT

		// Set options to wait for a maximum of 3 seconds for any new message to arrive
		gmo.Options |= ibmmq.MQGMO_WAIT
		gmo.WaitInterval = 10 * 1000 // The WaitInterval is in milliseconds

		// Create a buffer for the message data. This one is large enough
		// for the messages put by the amqsput sample.
		buffer := make([]byte, 1024)

		// Now we can try to get the message
		_, err := qObject.Get(getmqmd, gmo, buffer)
		if err != nil {
			mqret := err.(*ibmmq.MQReturn)
			if mqret.MQRC == ibmmq.MQRC_NO_MSG_AVAILABLE {
				continue
			}
			break
		}

		wg.Add(1)
		go func(gomd *ibmmq.MQMD) {
			log.Println("New message: ", gomd)
			err := sendMessage(cloudEventsClient, gomd)
			if err != nil {
				log.Printf("Failed to send message: %v\n", err)
			}
			wg.Done()
		}(getmqmd)
	}
}

// Disconnect from the queue manager
func disc(qMgrObject ibmmq.MQQueueManager) error {
	err := qMgrObject.Disc()
	if err == nil {
		fmt.Printf("Disconnected from queue manager %s\n", qMgrObject.Name)
	} else {
		fmt.Println(err)
	}
	return err
}

// Close the queue if it was opened
func close(object ibmmq.MQObject) error {
	err := object.Close(0)
	if err == nil {
		fmt.Println("Closed queue")
	} else {
		fmt.Println(err)
	}
	return err
}

func sendMessage(client *cloudevents.Client, message *ibmmq.MQMD) error {
	err := client.Send(message)
	if err != nil {
		return err
	}
	return nil
}
