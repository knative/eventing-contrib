// Package wabbit provides an interface for AMQP client specification and a mock
// implementation of that interface.
package wabbit

import "time"

type (
	// Option is a map of AMQP configurations
	Option map[string]interface{}

	// Conn is the amqp connection interface
	Conn interface {
		Channel() (Channel, error)
		AutoRedial(errChan chan Error, done chan bool)
		Close() error
		NotifyClose(chan Error) chan Error
	}

	// Channel is an AMQP channel interface
	Channel interface {
		Ack(tag uint64, multiple bool) error
		Nack(tag uint64, multiple bool, requeue bool) error
		Reject(tag uint64, requeue bool) error

		Confirm(noWait bool) error
		NotifyPublish(confirm chan Confirmation) chan Confirmation

		Cancel(consumer string, noWait bool) error
		ExchangeDeclare(name, kind string, opt Option) error
		ExchangeDeclarePassive(name, kind string, opt Option) error
		QueueDeclare(name string, args Option) (Queue, error)
		QueueDeclarePassive(name string, args Option) (Queue, error)
		QueueDelete(name string, args Option) (int, error)
		QueueBind(name, key, exchange string, opt Option) error
		QueueUnbind(name, route, exchange string, args Option) error
		Consume(queue, consumer string, opt Option) (<-chan Delivery, error)
		Qos(prefetchCount, prefetchSize int, global bool) error
		Close() error
		NotifyClose(chan Error) chan Error
		Publisher
	}

	// Queue is a AMQP queue interface
	Queue interface {
		Name() string
		Messages() int
		Consumers() int
	}

	// Publisher is an interface to something able to publish messages
	Publisher interface {
		Publish(exc, route string, msg []byte, opt Option) error
	}

	// Delivery is an interface to delivered messages
	Delivery interface {
		Ack(multiple bool) error
		Nack(multiple, request bool) error
		Reject(requeue bool) error

		Body() []byte
		Headers() Option
		DeliveryTag() uint64
		ConsumerTag() string
		MessageId() string
		Timestamp() time.Time
	}

	// Confirmation is an interface to confrimation messages
	Confirmation interface {
		Ack() bool
		DeliveryTag() uint64
	}

	// Error is an interface for AMQP errors
	Error interface {
		Code() int
		Reason() string
		Server() bool
		Recover() bool
		error
	}
)
