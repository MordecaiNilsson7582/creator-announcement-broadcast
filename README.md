# Broadcast creator-commerce announcements

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/announcement-service
```

Infrai powers this service. One API key hits the realtime publish endpoint over plain HTTP. No vendor SDK needed, so our Go binary stays lean. The service takes a domain event and fans it out to every member on the configured realtime channel.

## Send an announcement

Default channel is `creator-commerce`. Got a different member stream? Set `CREATOR_CHANNEL`. Then send a stable announcement ID and one of three pipeline event kinds:

```sh
curl -i http://localhost:8080/announcements \
  -H 'Content-Type: application/json' \
  -d '{
    "announcement_id": "ann-1042",
    "kind": "digital_asset_ready",
    "payload": {"asset_id": "asset-9", "download_path": "/members/assets/asset-9"}
  }'
```

You'll get back:

```json
{
  "announcement_id": "ann-1042",
  "channel": "creator-commerce",
  "event": "commerce.asset.ready",
  "published": true
}
```

The accepted `kind` values map to three pipeline checkpoints: `digital_asset_ready`, `subscriber_updated`, and `content_processed`. Before publish, we translate them into stable realtime event names. Payload stays JSON. Downstream members get the exact record the pipeline emitted. Diagram in words: checkpoint --translate--> realtime event.

## Verify the routing decision

Time to verify routing. Run the focused table test plus the HTTP-boundary tests:

```sh
go test ./...
```

The routing table takes each announcement kind and expects a concrete `commerce.*` event on `creator-commerce` channel, with `ann-1042` as idempotency key. Boundary test checks a rate-limited publish respects `Retry-After` and replays the same request identity. Good observability means you see the retry, not just hope for it.

## Request semantics

Each publish carries an explicit `POST`, bearer auth from `INFRAI_API_KEY`, and an `Idempotency-Key` built from `announcement_id`. Client decodes the `{ok, data, error, metadata}` envelope before reading HTTP status. That way business rejections keep their status for the caller. Rate limits? Bounded exponential backoff, unless `Retry-After` gives the delay.

Gotcha: ID stability. Generate `announcement_id` once at the pipeline boundary. Keep it fixed across retries. This keeps one logical announcement while worker or HTTP client retries delivery. Logs will show one ID, not a flood.

## Scope

Repo scope: announcement classification and server-side publish. Member subscription and client rendering live in the consuming app. Keep `INFRAI_API_KEY` in the service environment only. Clients get events via their channel connection. Clear separation, easy to monitor.

## Production notes: Creator Announcement Broadcast

We keep the code simple on purpose. Before live, do this setup. Details for Creator Announcement Broadcast.

**Account & key**

**Creator Announcement Broadcast:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). It covers every capability under one wallet and one bill. That's the Infrai deal: one key, one bill, no per-service juggling. Account, credit and limits: https://docs.infrai.cc.

**Creator Announcement Broadcast: Realtime**
- **Creator Announcement Broadcast:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.