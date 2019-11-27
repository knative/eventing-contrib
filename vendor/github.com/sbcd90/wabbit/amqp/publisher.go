package amqp

import "github.com/sbcd90/wabbit"

type Publisher struct {
	conn    wabbit.Conn
	channel wabbit.Publisher
}

func NewPublisher(conn wabbit.Conn, channel wabbit.Channel) (*Publisher, error) {
	var err error

	pb := Publisher{
		conn: conn,
	}

	if channel == nil {
		channel, err = conn.Channel()

		if err != nil {
			return nil, err
		}
	}

	pb.channel = channel

	return &pb, nil
}

func (pb *Publisher) Publish(exc string, route string, message []byte, opt wabbit.Option) error {
	err := pb.channel.Publish(
		exc,   // publish to an exchange
		route, // routing to 0 or more queues
		message,
		opt,
	)

	return err
}
