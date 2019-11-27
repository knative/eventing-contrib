package amqp

import (
	"time"

	"github.com/sbcd90/wabbit"
	"github.com/sbcd90/wabbit/utils"
	"github.com/streadway/amqp"
)

// Conn is the amqp connection
type Conn struct {
	*amqp.Connection

	// closure info of connection
	dialFn   func() error
	attempts uint8
}

// Dial to AMQP broker
func Dial(uri string) (*Conn, error) {
	conn := &Conn{}

	// closure the uri for handle reconnects
	conn.dialFn = func() error {
		var err error

		conn.Connection, err = amqp.Dial(uri)

		if err != nil {
			return err
		}

		return nil
	}

	err := conn.dialFn()

	if err != nil {
		return nil, err
	}

	return conn, nil
}

// NotifyClose registers a listener for close events.
// For more information see: https://godoc.org/github.com/streadway/amqp#Connection.NotifyClose
func (conn *Conn) NotifyClose(c chan wabbit.Error) chan wabbit.Error {
	amqpErr := conn.Connection.NotifyClose(make(chan *amqp.Error, cap(c)))

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

// AutoRedial manages the automatic redial of connection when unexpected closed.
// outChan is an unbuffered channel required to receive the errors that results from
// attempts of reconnect. On successfully reconnected, the true value is sent to done channel
//
// The outChan parameter can receive *amqp.Error for AMQP connection errors
// or errors.Error for any other net/tcp internal error.
//
// Redial strategy:
// If the connection is closed in an unexpected way (opposite of conn.Close()), then
// AutoRedial will try to automatically reconnect waiting for N seconds before each
// attempt, where N is the number of attempts of reconnecting. If the number of
// attempts reach 60, it will be zero'ed.
func (conn *Conn) AutoRedial(outChan chan wabbit.Error, done chan bool) {
	errChan2 := make(chan wabbit.Error)
	errChan := conn.NotifyClose(errChan2)

	go func() {
		var err wabbit.Error

		select {
		case amqpErr := <-errChan:
			err = amqpErr

			if amqpErr == nil {
				// Gracefull connection close
				return
			}
		attempts:
			outChan <- err

			if conn.attempts > 60 {
				conn.attempts = 0
			}

			// Wait n Seconds where n == conn.attempts...
			time.Sleep(time.Duration(conn.attempts) * time.Second)

			connErr := conn.dialFn()

			if connErr != nil {
				conn.attempts++
				goto attempts
			}

			// enabled AutoRedial on the new connection
			conn.AutoRedial(outChan, done)
			done <- true
			return
		}
	}()
}

// Channel returns a new channel ready to be used
func (conn *Conn) Channel() (wabbit.Channel, error) {
	ch, err := conn.Connection.Channel()

	if err != nil {
		return nil, err
	}

	return &Channel{ch}, nil
}
