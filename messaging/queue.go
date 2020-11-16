// Package messaging contains middleware for communication with decoupled
// services, could be RabbitMQ drivers as well as kafka or redis
package messaging

// Producer defines a producer behavior, exposes a single `Produce` method
// meant to enqueue an array of bytes
type Producer interface {
	Produce([]byte) error
}

// Consumer defines a consumer behavior, exposes a single `Consume` method
// meant to connect to a queue blocking while consuming incoming arrays of
// bytes forwarding them into a channel
type Consumer interface {
	Consume(chan<- []byte) error
}

// ProducerConsumer defines the behavior of a simple message queue, it's
// expected to provide a `Produce` function a `Consume` one
type ProducerConsumer interface {
	Producer
	Consumer
}

// ProducerConsumerCloser defines the behavior of a simple mssage queue
// that requires some kidn of external connection to be managed
type ProducerConsumerCloser interface {
	ProducerConsumer
	Close()
}
