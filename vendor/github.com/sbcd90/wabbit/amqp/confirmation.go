package amqp

import "github.com/streadway/amqp"

type Confirmation struct {
	amqp.Confirmation
}

func (c Confirmation) Ack() bool {
	return c.Confirmation.Ack
}

func (c Confirmation) DeliveryTag() uint64 {
	return c.Confirmation.DeliveryTag
}
