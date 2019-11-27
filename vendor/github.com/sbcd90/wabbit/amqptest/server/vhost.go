package server

import (
	"fmt"

	"github.com/sbcd90/wabbit"
)

// VHost is a fake AMQP virtual host
type VHost struct {
	name      string
	exchanges map[string]Exchange
	queues    map[string]*Queue
}

// NewVHost create a new fake AMQP Virtual Host
func NewVHost(name string) *VHost {
	vh := VHost{
		name:      name,
		queues:    make(map[string]*Queue),
		exchanges: make(map[string]Exchange),
	}

	vh.createDefaultExchanges()
	return &vh
}

func (v *VHost) createDefaultExchanges() {
	exchs := make(map[string]Exchange)
	exchs["amq.topic"] = &TopicExchange{}
	exchs["amq.direct"] = &DirectExchange{}
	exchs["topic"] = &TopicExchange{}
	exchs["direct"] = &DirectExchange{}
	exchs[""] = &DirectExchange{
		name: "amq.direct",
	}

	v.exchanges = exchs
}

func (v *VHost) Cancel(consumer string, noWait bool) error {
	return nil
}

// Qos isn't implemented in the fake server
func (v *VHost) Qos(prefetchCount, prefetchSize int, global bool) error {
	// do nothing. It's a implementation-specific tuning
	return nil
}

func (v *VHost) ExchangeDeclare(name, kind string, opt wabbit.Option) error {
	return v.exchangeDeclare(name, kind, false, opt)
}

func (v *VHost) ExchangeDeclarePassive(name, kind string, opt wabbit.Option) error {
	return v.exchangeDeclare(name, kind, true, opt)
}

func (v *VHost) exchangeDeclare(name, kind string, passive bool, opt wabbit.Option) error {
	if _, ok := v.exchanges[name]; ok {
		// TODO: We need review this. If the application is trying to re-create an exchange
		// using other options we shall not return NIL because this indicates success,
		// but we didn't declared anything.
		// The AMQP 0.9.1 spec says nothing about that. It only says that AMQP uses the
		// "declare" concept instead of the "create" concept. If something is already
		// declared it's no problem...
		return nil
	}

	if passive {
		return fmt.Errorf("Exception (404) Reason: \"NOT_FOUND - no exchange '%s' in vhost '%s'\"", name, v.name)
	}

	switch kind {
	case "topic":
		v.exchanges[name] = NewTopicExchange(name)
	case "direct":
		v.exchanges[name] = NewDirectExchange(name)
	default:
		return fmt.Errorf("Invalid exchange type: %s", kind)
	}

	return nil
}

func (v *VHost) QueueDeclare(name string, args wabbit.Option) (wabbit.Queue, error) {
	return v.queueDeclare(name, false, args)
}

func (v *VHost) QueueDeclarePassive(name string, args wabbit.Option) (wabbit.Queue, error) {
	return v.queueDeclare(name, true, args)
}

func (v *VHost) queueDeclare(name string, passive bool, args wabbit.Option) (wabbit.Queue, error) {
	if q, ok := v.queues[name]; ok {
		return q, nil
	}

	if passive {
		return nil, fmt.Errorf("Exception (404) Reason: \"NOT_FOUND - no queue '%s' in vhost '%s'\"", name, v.name)
	}

	q := NewQueue(name)

	v.queues[name] = q

	err := v.QueueBind(name, name, "", nil)

	if err != nil {
		return nil, err
	}

	return q, nil
}

func (v *VHost) QueueDelete(name string, args wabbit.Option) (int, error) {
	delete(v.queues, name)
	return 0, nil
}

func (v *VHost) QueueBind(name, key, exchange string, _ wabbit.Option) error {
	var (
		exch Exchange
		q    *Queue
		ok   bool
	)

	if exch, ok = v.exchanges[exchange]; !ok {
		return fmt.Errorf("Unknown exchange '%s'", exchange)
	}

	if q, ok = v.queues[name]; !ok {
		return fmt.Errorf("Unknown queue '%s'", name)
	}

	exch.addBinding(key, q)
	return nil
}

func (v *VHost) QueueUnbind(name, key, exchange string, _ wabbit.Option) error {
	var (
		exch Exchange
		ok   bool
	)

	if exch, ok = v.exchanges[exchange]; !ok {
		return fmt.Errorf("Unknown exchange '%s'", exchange)
	}

	if _, ok = v.queues[name]; !ok {
		return fmt.Errorf("Unknown queue '%s'", name)
	}

	exch.delBinding(key)
	return nil
}

// Publish push a new message to queue data channel.
// The queue data channel is a buffered channel of length `QueueMaxLen`. If
// the queue is full, this method will block until some messages are consumed.
func (v *VHost) Publish(exc, route string, d *Delivery, _ wabbit.Option) error {
	var (
		exch Exchange
		ok   bool
		err  error
	)

	if exch, ok = v.exchanges[exc]; !ok {
		return fmt.Errorf("Unknow exchange '%s'", exc)
	}

	err = exch.route(route, d)

	if err != nil {
		return err
	}

	return nil
}
