package amqp

import (
	"github.com/sbcd90/wabbit"
	"github.com/streadway/amqp"
	"time"
)

type Delivery struct {
	*amqp.Delivery
}

func (d *Delivery) Body() []byte {
	return d.Delivery.Body
}

func (d *Delivery) Headers() wabbit.Option {
	return wabbit.Option(d.Delivery.Headers)
}

func (d *Delivery) DeliveryTag() uint64 {
	return d.Delivery.DeliveryTag
}

func (d *Delivery) ConsumerTag() string {
	return d.Delivery.ConsumerTag
}

func (d *Delivery) MessageId() string {
	return d.Delivery.MessageId
}

func (d *Delivery) Timestamp() time.Time {
	return d.Delivery.Timestamp
}
