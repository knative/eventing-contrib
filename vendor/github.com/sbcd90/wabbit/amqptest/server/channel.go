package server

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/sbcd90/wabbit"
	"github.com/sbcd90/wabbit/utils"
	"github.com/streadway/amqp"
)

type (
	Channel struct {
		*VHost

		unacked    []unackData
		muUnacked  *sync.RWMutex
		consumers  map[string]consumer
		muConsumer *sync.RWMutex

		_                  uint32
		deliveryTagCounter uint64

		confirm bool

		publishListeners   []chan wabbit.Confirmation
		muPublishListeners *sync.RWMutex

		errSpread *utils.ErrBroadcast
	}

	unackData struct {
		d wabbit.Delivery
		q *Queue
	}

	consumer struct {
		tag        string
		deliveries chan wabbit.Delivery
		done       chan bool
	}
)

var consumerSeq uint64

func uniqueConsumerTag() string {
	return fmt.Sprintf("ctag-%s-%d", os.Args[0], atomic.AddUint64(&consumerSeq, 1))
}

func NewChannel(vhost *VHost) *Channel {
	c := Channel{
		VHost:              vhost,
		unacked:            make([]unackData, 0, QueueMaxLen),
		muUnacked:          &sync.RWMutex{},
		muConsumer:         &sync.RWMutex{},
		consumers:          make(map[string]consumer),
		muPublishListeners: &sync.RWMutex{},
		errSpread:          utils.NewErrBroadcast(),
	}

	return &c
}

func (ch *Channel) Confirm(noWait bool) error {
	ch.confirm = true

	return nil
}

func (ch *Channel) NotifyPublish(confirm chan wabbit.Confirmation) chan wabbit.Confirmation {
	aux := make(chan wabbit.Confirmation, 2<<8)

	ch.muPublishListeners.Lock()
	ch.publishListeners = append(ch.publishListeners, aux)
	ch.muPublishListeners.Unlock()

	// aux is set to the maximum size of a queue: 512.
	// In theory it is possible that a publisher could deliver >512 messages
	// on a channel by sending to multiple queues, in which case Publish()
	// will block trying to send confirmations. However this is a pathological
	// case which can be ignored since there should be an attached listener
	// on the other end of the confirm channel. In any case, a buffered queue,
	// while seemingly inefficient is a good enough solution to make sure
	// Publish() doesn't block.
	go func() {
		for c := range aux {
			confirm <- c
		}
		close(confirm)
	}()

	return confirm
}

func (ch *Channel) Publish(exc, route string, msg []byte, opt wabbit.Option) error {
	hdrs, _ := opt["headers"].(amqp.Table)
	messageId, _ := opt["messageId"].(string)
	d := NewDelivery(ch,
		msg,
		atomic.AddUint64(&ch.deliveryTagCounter, 1),
		messageId,
		wabbit.Option(hdrs))

	err := ch.VHost.Publish(exc, route, d, nil)

	if err != nil {
		return err
	}

	if ch.confirm {
		confirm := Confirmation{ch.deliveryTagCounter, true}
		for _, l := range ch.publishListeners {
			l <- confirm
		}
	}

	return nil
}

// Consume starts a fake consumer of queue
func (ch *Channel) Consume(queue, consumerName string, _ wabbit.Option) (<-chan wabbit.Delivery, error) {
	var (
		c consumer
	)

	if consumerName == "" {
		consumerName = uniqueConsumerTag()
	}

	c = consumer{
		tag:        consumerName,
		deliveries: make(chan wabbit.Delivery),
		done:       make(chan bool),
	}

	ch.muConsumer.RLock()

	if c2, found := ch.consumers[consumerName]; found {
		c2.done <- true
	}

	ch.consumers[consumerName] = c

	ch.muConsumer.RUnlock()

	q, ok := ch.queues[queue]

	if !ok {
		return nil, fmt.Errorf("Unknown queue '%s'", queue)
	}

	go func() {
		for {
			select {
			case <-c.done:
				close(c.deliveries)
				return
			case d := <-q.data:
				// since we keep track of unacked messages for
				// the channel, we need to rebind the delivery
				// to the consumer channel.
				d = NewDelivery(ch, d.Body(), d.DeliveryTag(), d.MessageId(), d.Headers())

				ch.addUnacked(d, q)

				// sub-select required for cases when
				// client attempts to close the channel
				// concurrently with re-enqueues of messages
				select {
				case c.deliveries <- d:
				case <-c.done:
					close(c.deliveries)
					return
				}
			}
		}
	}()

	return c.deliveries, nil
}

func (ch *Channel) addUnacked(d wabbit.Delivery, q *Queue) {
	ch.muUnacked.Lock()
	defer ch.muUnacked.Unlock()

	ch.unacked = append(ch.unacked, unackData{d, q})
}

func (ch *Channel) enqueueUnacked() {
	ch.muUnacked.Lock()
	defer ch.muUnacked.Unlock()

	for _, ud := range ch.unacked {
		ud.q.data <- ud.d
	}

	ch.unacked = make([]unackData, 0, QueueMaxLen)
}

func (ch *Channel) Ack(tag uint64, multiple bool) error {
	var (
		pos int
		ud  unackData
	)

	if !multiple {
		ch.muUnacked.Lock()
		defer ch.muUnacked.Unlock()

		found := false
		for pos, ud = range ch.unacked {
			if ud.d.DeliveryTag() == tag {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("Delivery tag %d not found", tag)
		}

		ch.unacked = ch.unacked[:pos+copy(ch.unacked[pos:], ch.unacked[pos+1:])]
	} else {
		ackMessages := make([]uint64, 0, QueueMaxLen)

		ch.muUnacked.Lock()

		found := false
		for _, ud = range ch.unacked {
			udTag := ud.d.DeliveryTag()

			if udTag <= tag {
				found = true
				ackMessages = append(ackMessages, udTag)
			}
		}

		ch.muUnacked.Unlock()

		if !found {
			return fmt.Errorf("Delivery tag %d not found", tag)
		}

		for _, udTag := range ackMessages {
			ch.Ack(udTag, false)
		}
	}

	return nil
}

func (ch *Channel) Nack(tag uint64, multiple bool, requeue bool) error {
	var (
		pos int
		ud  unackData
	)

	if !multiple {
		found := false
		for pos, ud = range ch.unacked {
			if ud.d.DeliveryTag() == tag {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("Delivery tag %d not found", tag)
		}

		if requeue {
			ud.q.data <- ud.d
		}

		ch.muUnacked.Lock()
		ch.unacked = ch.unacked[:pos+copy(ch.unacked[pos:], ch.unacked[pos+1:])]
		ch.muUnacked.Unlock()
	} else {
		nackMessages := make([]uint64, 0, QueueMaxLen)

		for _, ud = range ch.unacked {
			udTag := ud.d.DeliveryTag()

			if udTag <= tag {
				nackMessages = append(nackMessages, udTag)
			}
		}

		for _, udTag := range nackMessages {
			ch.Nack(udTag, false, requeue)
		}
	}

	return nil
}

func (ch *Channel) Reject(tag uint64, requeue bool) error {
	return ch.Nack(tag, false, requeue)
}

func (ch *Channel) Close() error {
	ch.muConsumer.Lock()
	defer ch.muConsumer.Unlock()

	for _, consumer := range ch.consumers {
		consumer.done <- true
	}

	ch.consumers = make(map[string]consumer)

	// enqueue shall happens only after every consumer of this channel
	// has stopped.
	ch.enqueueUnacked()

	ch.muPublishListeners.Lock()
	defer ch.muPublishListeners.Unlock()
	for _, c := range ch.publishListeners {
		close(c)
	}
	ch.publishListeners = []chan wabbit.Confirmation{}

	return nil
}

// NotifyClose publishs notifications about errors in the given channel
func (ch *Channel) NotifyClose(c chan wabbit.Error) chan wabbit.Error {
	ch.errSpread.Add(c)
	return c
}

// Cancel closes deliveries for all consumers
func (ch *Channel) Cancel(consumer string, noWait bool) error {
	ch.Close()
	return nil
}
