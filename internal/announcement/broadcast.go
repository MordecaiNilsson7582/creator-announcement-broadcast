package announcement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type Kind string

const (
	DigitalAssetReady Kind = "digital_asset_ready"
	SubscriberUpdated Kind = "subscriber_updated"
	ContentProcessed  Kind = "content_processed"
)

type Input struct {
	AnnouncementID string          `json:"announcement_id"`
	Kind           Kind            `json:"kind"`
	Payload        json.RawMessage `json:"payload"`
}

type Result struct {
	AnnouncementID string `json:"announcement_id"`
	Channel        string `json:"channel"`
	Event          string `json:"event"`
	Published      bool   `json:"published"`
}

type Publisher interface {
	Publish(ctx context.Context, idempotencyKey, channel, event string, data json.RawMessage) error
}

type Broadcaster struct {
	channel   string
	publisher Publisher
}

func NewBroadcaster(channel string, publisher Publisher) (*Broadcaster, error) {
	if channel == "" {
		return nil, errors.New("channel is required")
	}
	if publisher == nil {
		return nil, errors.New("publisher is required")
	}
	return &Broadcaster{channel: channel, publisher: publisher}, nil
}

func (b *Broadcaster) Broadcast(ctx context.Context, in Input) (Result, error) {
	if in.AnnouncementID == "" {
		return Result{}, errors.New("announcement_id is required")
	}
	if len(in.Payload) == 0 || !json.Valid(in.Payload) {
		return Result{}, errors.New("payload must be valid JSON")
	}

	event, err := eventFor(in.Kind)
	if err != nil {
		return Result{}, err
	}
	if err := b.publisher.Publish(ctx, in.AnnouncementID, b.channel, event, in.Payload); err != nil {
		return Result{}, err
	}
	return Result{
		AnnouncementID: in.AnnouncementID,
		Channel:        b.channel,
		Event:          event,
		Published:      true,
	}, nil
}

func eventFor(kind Kind) (string, error) {
	switch kind {
	case DigitalAssetReady:
		return "commerce.asset.ready", nil
	case SubscriberUpdated:
		return "commerce.subscriber.updated", nil
	case ContentProcessed:
		return "commerce.content.processed", nil
	default:
		return "", fmt.Errorf("unknown announcement kind %q", kind)
	}
}
