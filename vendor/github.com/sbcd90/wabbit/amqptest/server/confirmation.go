package server

type Confirmation struct {
	deliveryTag uint64
	ack         bool
}

func (c Confirmation) Ack() bool {
	return c.ack
}

func (c Confirmation) DeliveryTag() uint64 {
	return c.deliveryTag
}
