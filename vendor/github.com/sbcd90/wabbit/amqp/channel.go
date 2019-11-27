package amqp

import (
	"errors"

	"github.com/sbcd90/wabbit"
	"github.com/sbcd90/wabbit/utils"
	"github.com/streadway/amqp"
)

// Channel is a wrapper channel structure for amqp.Channel
type Channel struct {
	*amqp.Channel
}

func (ch *Channel) Publish(exc, route string, msg []byte, opt wabbit.Option) error {
	amqpOpt, err := utils.ConvertOpt(opt)

	if err != nil {
		return err
	}

	amqpOpt.Body = msg

	return ch.Channel.Publish(
		exc,   // publish to an exchange
		route, // routing to 0 or more queues
		false, // mandatory
		false, // immediate
		amqpOpt,
	)
}

func (ch *Channel) Confirm(noWait bool) error {
	return ch.Channel.Confirm(noWait)
}

func (ch *Channel) NotifyPublish(confirm chan wabbit.Confirmation) chan wabbit.Confirmation {
	amqpConfirms := ch.Channel.NotifyPublish(make(chan amqp.Confirmation, cap(confirm)))

	go func() {
		for c := range amqpConfirms {
			confirm <- Confirmation{c}
		}

		close(confirm)
	}()

	return confirm
}

func (ch *Channel) Consume(queue, consumer string, opt wabbit.Option) (<-chan wabbit.Delivery, error) {
	var (
		autoAck, exclusive, noLocal, noWait bool
		args                                amqp.Table
	)

	if v, ok := opt["autoAck"]; ok {
		autoAck, ok = v.(bool)

		if !ok {
			return nil, errors.New("durable option is of type bool")
		}
	}

	if v, ok := opt["exclusive"]; ok {
		exclusive, ok = v.(bool)

		if !ok {
			return nil, errors.New("exclusive option is of type bool")
		}
	}

	if v, ok := opt["noLocal"]; ok {
		noLocal, ok = v.(bool)

		if !ok {
			return nil, errors.New("noLocal option is of type bool")
		}
	}

	if v, ok := opt["noWait"]; ok {
		noWait, ok = v.(bool)

		if !ok {
			return nil, errors.New("noWait option is of type bool")
		}
	}

	if v, ok := opt["args"]; ok {
		args, ok = v.(amqp.Table)

		if !ok {
			return nil, errors.New("args is of type amqp.Table")
		}
	}

	amqpd, err := ch.Channel.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)

	if err != nil {
		return nil, err
	}

	deliveries := make(chan wabbit.Delivery)

	go func() {
		for d := range amqpd {
			delivery := d
			deliveries <- &Delivery{&delivery}
		}

		close(deliveries)
	}()

	return deliveries, nil
}

func (ch *Channel) ExchangeDeclare(name, kind string, opt wabbit.Option) error {
	return ch.exchangeDeclare(name, kind, false, opt)
}

func (ch *Channel) ExchangeDeclarePassive(name, kind string, opt wabbit.Option) error {
	return ch.exchangeDeclare(name, kind, true, opt)
}

func (ch *Channel) exchangeDeclare(name, kind string, passive bool, opt wabbit.Option) error {
	var (
		durable, autoDelete, internal, noWait bool
		args                                  amqp.Table
	)

	if v, ok := opt["durable"]; ok {
		durable, ok = v.(bool)

		if !ok {
			return errors.New("durable option is of type bool")
		}
	}

	if v, ok := opt["autoDelete"]; ok {
		autoDelete, ok = v.(bool)

		if !ok {
			return errors.New("autoDelete option is of type bool")
		}
	}

	if v, ok := opt["internal"]; ok {
		internal, ok = v.(bool)

		if !ok {
			return errors.New("internal option is of type bool")
		}
	}

	if v, ok := opt["noWait"]; ok {
		noWait, ok = v.(bool)

		if !ok {
			return errors.New("noWait option is of type bool")
		}
	}

	if v, ok := opt["args"]; ok {
		args, ok = v.(amqp.Table)

		if !ok {
			return errors.New("args is of type amqp.Table")
		}
	}
	if passive {
		return ch.Channel.ExchangeDeclarePassive(name, kind, durable, autoDelete, internal, noWait, args)
	}
	return ch.Channel.ExchangeDeclare(name, kind, durable, autoDelete, internal, noWait, args)
}

func (ch *Channel) QueueUnbind(name, route, exchange string, _ wabbit.Option) error {
	return ch.Channel.QueueUnbind(name, route, exchange, nil)
}

// QueueBind binds the route key to queue
func (ch *Channel) QueueBind(name, key, exchange string, opt wabbit.Option) error {
	var (
		noWait bool
		args   amqp.Table
	)

	if v, ok := opt["noWait"]; ok {
		noWait, ok = v.(bool)

		if !ok {
			return errors.New("noWait option is of type bool")
		}
	}

	if v, ok := opt["args"]; ok {
		args, ok = v.(amqp.Table)

		if !ok {
			return errors.New("args is of type amqp.Table")
		}
	}

	return ch.Channel.QueueBind(name, key, exchange, noWait, args)
}

// QueueDeclare declares a new AMQP queue
func (ch *Channel) QueueDeclare(name string, opt wabbit.Option) (wabbit.Queue, error) {
	return ch.queueDeclare(name, false, opt)
}

// QueueDeclarePassive declares an existing AMQP queue
func (ch *Channel) QueueDeclarePassive(name string, opt wabbit.Option) (wabbit.Queue, error) {
	return ch.queueDeclare(name, true, opt)
}

func (ch *Channel) queueDeclare(name string, passive bool, opt wabbit.Option) (wabbit.Queue, error) {
	var (
		durable, autoDelete, exclusive, noWait bool
		args                                   amqp.Table
	)

	if v, ok := opt["durable"]; ok {
		durable, ok = v.(bool)

		if !ok {
			return nil, errors.New("durable option is of type bool")
		}
	}

	if v, ok := opt["autoDelete"]; ok {
		autoDelete, ok = v.(bool)

		if !ok {
			return nil, errors.New("autoDelete option is of type bool")
		}
	}

	if v, ok := opt["exclusive"]; ok {
		exclusive, ok = v.(bool)

		if !ok {
			return nil, errors.New("Exclusive option is of type bool")
		}
	}

	if v, ok := opt["noWait"]; ok {
		noWait, ok = v.(bool)

		if !ok {
			return nil, errors.New("noWait option is of type bool")
		}
	}

	if v, ok := opt["args"]; ok {
		args, ok = v.(amqp.Table)

		if !ok {
			return nil, errors.New("args is of type amqp.Table")
		}
	}

	var q amqp.Queue
	var err error
	if passive {
		q, err = ch.Channel.QueueDeclarePassive(name, durable, autoDelete, exclusive, noWait, args)
	} else {
		q, err = ch.Channel.QueueDeclare(name, durable, autoDelete, exclusive, noWait, args)
	}

	if err != nil {
		return nil, err
	}

	return &Queue{&q}, nil
}

func (ch *Channel) QueueDelete(name string, opt wabbit.Option) (int, error) {
	var (
		ifUnused, ifEmpty, noWait bool
	)

	if v, ok := opt["ifUnused"]; ok {
		ifUnused, ok = v.(bool)

		if !ok {
			return 0, errors.New("ifUnused option is of type bool")
		}
	}

	if v, ok := opt["ifEmpty"]; ok {
		ifEmpty, ok = v.(bool)

		if !ok {
			return 0, errors.New("ifEmpty option is of type bool")
		}
	}

	if v, ok := opt["noWait"]; ok {
		noWait, ok = v.(bool)

		if !ok {
			return 0, errors.New("noWait option is of type bool")
		}
	}

	return ch.Channel.QueueDelete(name, ifUnused, ifEmpty, noWait)
}

// Qos controls how many bytes or messages will be handled by channel or connection.
func (ch *Channel) Qos(prefetchCount, prefetchSize int, global bool) error {
	return ch.Channel.Qos(prefetchCount, prefetchSize, global)
}

// NotifyClose registers a listener for close events.
// For more information see: https://godoc.org/github.com/streadway/amqp#Channel.NotifyClose
func (ch *Channel) NotifyClose(c chan wabbit.Error) chan wabbit.Error {
	amqpErr := ch.Channel.NotifyClose(make(chan *amqp.Error, cap(c)))

	go func() {
		for err := range amqpErr {
			var ne wabbit.Error
			if err != nil {
				ne = utils.NewError(
					err.Code,
					err.Reason,
					err.Server,
					err.Recover,
				)
			} else {
				ne = nil
			}
			c <- ne
		}
		close(c)
	}()

	return c
}
