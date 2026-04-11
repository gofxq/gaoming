package redis

import (
	"context"
	"encoding/json"

	"github.com/gofxq/gaoming/pkg/hostruntime/repository"
	"github.com/gofxq/gaoming/pkg/state"
	goredis "github.com/redis/go-redis/v9"
)

type EventBus struct {
	client  *goredis.Client
	channel string
}

type eventEnvelope struct {
	Type     repository.HostEventType `json:"type"`
	HostUID  string                   `json:"host_uid"`
	Snapshot *state.HostSnapshot      `json:"snapshot,omitempty"`
}

func NewEventBus(client *goredis.Client, channel string) *EventBus {
	if channel == "" {
		channel = "gaoming:master-api:host-events"
	}
	return &EventBus{client: client, channel: channel}
}

func (b *EventBus) PublishHostUpsert(ctx context.Context, snapshot state.HostSnapshot) error {
	body, err := json.Marshal(eventEnvelope{
		Type:     repository.HostEventUpsert,
		HostUID:  snapshot.HostUID,
		Snapshot: &snapshot,
	})
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.channel, body).Err()
}

func (b *EventBus) PublishHostDelete(ctx context.Context, hostUID string) error {
	body, err := json.Marshal(eventEnvelope{
		Type:    repository.HostEventDelete,
		HostUID: hostUID,
	})
	if err != nil {
		return err
	}
	return b.client.Publish(ctx, b.channel, body).Err()
}

func (b *EventBus) SubscribeHostEvents(ctx context.Context) (<-chan repository.HostEvent, error) {
	pubsub := b.client.Subscribe(ctx, b.channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}

	out := make(chan repository.HostEvent, 64)
	go func() {
		defer close(out)
		defer pubsub.Close()

		msgCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgCh:
				if !ok {
					return
				}

				var env eventEnvelope
				if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
					continue
				}
				event := repository.HostEvent{
					Type:     env.Type,
					HostUID:  env.HostUID,
					Snapshot: env.Snapshot,
				}
				select {
				case out <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}
