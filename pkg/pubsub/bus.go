// Package pubsub provides ephemeral in-process publish/subscribe contracts.
package pubsub

import "context"

// Topic is the logical channel name used for fan-out.
type Topic string

// Message is the payload distributed to subscribers.
type Message struct {
	Topic   Topic
	Payload []byte
	Headers map[string]string
}

// Subscriber receives messages from a subscribed topic.
type Subscriber interface {
	Receive() <-chan Message
	Close() error
}

// Bus defines ephemeral in-process fan-out behavior.
type Bus interface {
	Publish(ctx context.Context, msg Message) error
	Subscribe(topic Topic) (Subscriber, error)
	Unsubscribe(topic Topic, sub Subscriber) error
	Close() error
}
