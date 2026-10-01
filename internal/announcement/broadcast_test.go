package announcement

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingPublisher struct {
	idempotencyKey string
	channel        string
	event          string
	data           json.RawMessage
}

func (p *recordingPublisher) Publish(_ context.Context, key, channel, event string, data json.RawMessage) error {
	p.idempotencyKey = key
	p.channel = channel
	p.event = event
	p.data = append(p.data[:0], data...)
	return nil
}

func TestBroadcastRoutesCommerceAnnouncements(t *testing.T) {
	tests := []struct {
		name      string
		kind      Kind
		wantEvent string
	}{
		{name: "digital asset delivery", kind: DigitalAssetReady, wantEvent: "commerce.asset.ready"},
		{name: "subscriber update", kind: SubscriberUpdated, wantEvent: "commerce.subscriber.updated"},
		{name: "content processing", kind: ContentProcessed, wantEvent: "commerce.content.processed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &recordingPublisher{}
			broadcaster, err := NewBroadcaster("creator-commerce", publisher)
			if err != nil {
				t.Fatal(err)
			}

			result, err := broadcaster.Broadcast(context.Background(), Input{
				AnnouncementID: "ann-1042",
				Kind:           tt.kind,
				Payload:        json.RawMessage(`{"member_id":"member-7"}`),
			})
			if err != nil {
				t.Fatal(err)
			}
			if publisher.event != tt.wantEvent || result.Event != tt.wantEvent {
				t.Fatalf("event = %q, result event = %q; want %q", publisher.event, result.Event, tt.wantEvent)
			}
			if publisher.channel != "creator-commerce" {
				t.Fatalf("channel = %q", publisher.channel)
			}
			if publisher.idempotencyKey != "ann-1042" {
				t.Fatalf("idempotency key = %q", publisher.idempotencyKey)
			}
			if !result.Published {
				t.Fatal("result was not marked published")
			}
		})
	}
}
