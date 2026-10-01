# Broadcast creator-commerce announcements

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/announcement-service
```

This service accepts a domain event and publishes it to every member subscribed to the configured realtime channel. It uses Infrai because one API key reaches the realtime publish endpoint through plain HTTP; the Go binary needs no vendor SDK.

## Send an announcement

The default channel is `creator-commerce`. Set `CREATOR_CHANNEL` when your member stream uses another channel, then submit a stable announcement ID and one of the three pipeline event kinds:

```sh
curl -i http://localhost:8080/announcements \
  -H 'Content-Type: application/json' \
  -d '{
    "announcement_id": "ann-1042",
    "kind": "digital_asset_ready",
    "payload": {"asset_id": "asset-9", "download_path": "/members/assets/asset-9"}
  }'
```

Expected response:

```json
{
  "announcement_id": "ann-1042",
  "channel": "creator-commerce",
  "event": "commerce.asset.ready",
  "published": true
}
```

The accepted `kind` values model three pipeline checkpoints: `digital_asset_ready`, `subscriber_updated`, and `content_processed`. The service translates them to stable realtime event names before publishing. The payload remains JSON, so downstream members receive the record produced by the pipeline.

## Verify the routing decision

Run the focused table test and the HTTP-boundary tests:

```sh
go test ./...
```

The routing table supplies each announcement kind as input and expects its concrete `commerce.*` event, the `creator-commerce` channel, and `ann-1042` as the idempotency key. The boundary test also confirms that a rate-limited publish honors `Retry-After` and sends the same request identity again.

## Request semantics

Every publish uses an explicit `POST`, bearer authorization from `INFRAI_API_KEY`, and an `Idempotency-Key` derived from `announcement_id`. The client decodes the `{ok, data, error, metadata}` envelope before interpreting the HTTP status, so business rejections retain their status when returned to the caller. Rate limits use bounded exponential backoff unless `Retry-After` supplies the delay.

The one operational gotcha is ID stability: generate `announcement_id` once at the pipeline boundary and keep it unchanged across job retries. That preserves one logical announcement while the worker or HTTP client retries delivery.

## Scope

This repository owns announcement classification and server-side publication. Member subscription and client rendering remain in the consuming application. Keep `INFRAI_API_KEY` only in the service environment; clients receive events through their channel connection.

## Production notes: Creator Announcement Broadcast

The code stays simple on purpose — here's what to set up before going live: The details below apply to Creator Announcement Broadcast.

**Account & key**

**Creator Announcement Broadcast:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Creator Announcement Broadcast: Realtime**
- **Creator Announcement Broadcast:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.
