package amqptest

type Queue struct {
	name      string
	consumers int
	messages  int
}

func NewQueue(name string) *Queue {
	return &Queue{
		name: name,
	}
}

func (q *Queue) Name() string {
	return q.name
}

func (q *Queue) Messages() int {
	return q.messages
}

func (q *Queue) Consumers() int {
	return q.consumers
}
