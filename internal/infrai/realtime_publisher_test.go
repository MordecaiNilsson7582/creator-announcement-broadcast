package infrai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPublishRetriesRateLimitWithSameBoundary(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodPost {
			t.Fatalf("method = %s", req.Method)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header = %q", req.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var sent map[string]json.RawMessage
		if err := json.Unmarshal(body, &sent); err != nil {
			t.Fatal(err)
		}
		if len(sent) != 4 || sent["channel"] == nil || sent["event"] == nil || sent["data"] == nil || sent["idempotency_key"] == nil {
			t.Fatalf("request fields = %s", body)
		}
		var key string
		if err := json.Unmarshal(sent["idempotency_key"], &key); err != nil || key != "ann-1042" {
			t.Fatalf("idempotency_key = %s", sent["idempotency_key"])
		}

		status := http.StatusOK
		responseBody := `{"ok":true,"data":{},"error":null,"metadata":{}}`
		header := make(http.Header)
		if requests == 1 {
			status = http.StatusTooManyRequests
			responseBody = `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`
			header.Set("Retry-After", "2")
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(responseBody))}, nil
	})}

	publisher, err := NewPublisher("test-key", client)
	if err != nil {
		t.Fatal(err)
	}
	var delays []time.Duration
	publisher.sleep = func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	err = publisher.Publish(context.Background(), "ann-1042", "creator-commerce", "commerce.asset.ready", json.RawMessage(`{"asset_id":"asset-9"}`))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d; want 2", requests)
	}
	if len(delays) != 1 || delays[0] != 2*time.Second {
		t.Fatalf("delays = %v", delays)
	}
}

func TestPublishReturnsBusinessEnvelopeBeforeHTTPStatus(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":false,"data":null,"error":{"code":"REJECTED","message":"invalid announcement"},"metadata":{}}`)),
		}, nil
	})}
	publisher, err := NewPublisher("test-key", client)
	if err != nil {
		t.Fatal(err)
	}

	err = publisher.Publish(context.Background(), "ann-1042", "creator-commerce", "commerce.asset.ready", json.RawMessage(`{}`))
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T %v", err, err)
	}
	if apiErr.Code != "REJECTED" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("API error = %+v", apiErr)
	}
}
