package pubsub

import (
	"cloud.google.com/go/pubsub"
	"context"
	"fmt"
)

// NewPubSubClient creates new client for a given projectID.
func NewPubSubClient(ctx context.Context, projectID string) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, projectID)
}

// CreateTopic creates topic.
func CreateTopic(ctx context.Context, client *pubsub.Client, topic string) error {
	_, err := client.CreateTopic(ctx, topic)
	if err != nil {
		return err
	}
	return nil
}

// CreateSubscription creates subscription for the topic.
func CreateSubscription(ctx context.Context, client *pubsub.Client, topic string, subs string) error {
	_, err := client.CreateSubscription(ctx, subs, pubsub.SubscriptionConfig{Topic: client.Topic(topic)})
	if err != nil {
		return err
	}
	return nil
}

// Publish the message to the topic.
func Publish(ctx context.Context, client *pubsub.Client, topic string, msg string, attr map[string]string) error {
	m := pubsub.Message{
		Data: []byte(msg),
	}
	if len(attr) != 0 {
		m.Attributes = attr
	}

	t := client.Topic(topic)
	defer t.Stop()

	r := t.Publish(ctx, &m)

	id, err := r.Get(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("published a message with a message ID: %s\n", id)
	return nil
}
