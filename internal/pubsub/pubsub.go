package pubsub

import (
	"cloud.google.com/go/pubsub"
	"context"
	"fmt"
)

func NewPubSubClient(ctx context.Context, projectID string) (*pubsub.Client, error) {
	return pubsub.NewClient(ctx, projectID)
}

func CreateTopic(ctx context.Context, client *pubsub.Client, topic string) error {
	_, err := client.CreateTopic(ctx, topic)
	return err
}

func CreateSubscription(ctx context.Context, client *pubsub.Client, topic string, subs string) error {
	_, err := client.CreateSubscription(ctx, subs, pubsub.SubscriptionConfig{Topic: client.Topic(topic)})
	return err
}

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
