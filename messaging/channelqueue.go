// Package messaging contains middleware for communication with decoupled
// services, could be RabbitMQ drivers as well as kafka or redis
package messaging

// ChannelQueue is a simple in-memory `ProducerConsumerCloser` implementation
// using a channel as backend
type ChannelQueue struct {
	bus chan []byte
}

// NewChannelQueue create a new ChannelQueue
func NewChannelQueue() ChannelQueue {
	return ChannelQueue{make(chan []byte)}
}

// Produce send a payload of bytes into the channel
func (c ChannelQueue) Produce(data []byte) error {
	c.bus <- data
	return nil
}

// Consume subscribes to the underlying ChannelQueue's channel forwarding all
// incoming events to a push-only channel
func (c ChannelQueue) Consume(events chan<- []byte) error {
	for event := range c.bus {
		events <- event
	}
	return nil
}

// Close close the underlying channel
func (c ChannelQueue) Close() {
	close(c.bus)
}
