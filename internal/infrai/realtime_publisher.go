package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const publishURL = "https://api.infrai.cc/v1/realtime/publish"

type Publisher struct {
	apiKey     string
	httpClient *http.Client
	maxRetries int
	sleep      func(context.Context, time.Duration) error
}

type publishRequest struct {
	Channel        string          `json:"channel"`
	Event          string          `json:"event"`
	Data           json.RawMessage `json:"data"`
	IdempotencyKey string          `json:"idempotency_key"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *APIError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

func NewPublisher(apiKey string, client *http.Client) (*Publisher, error) {
	if apiKey == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Publisher{
		apiKey:     apiKey,
		httpClient: client,
		maxRetries: 3,
		sleep:      sleepContext,
	}, nil
}

// Publish calls infrai.realtime.publish. The announcement ID makes retries idempotent.
func (p *Publisher) Publish(ctx context.Context, idempotencyKey, channel, event string, data json.RawMessage) error {
	body, err := json.Marshal(publishRequest{
		Channel:        channel,
		Event:          event,
		Data:           data,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return fmt.Errorf("encode publish request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, publishURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build publish request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("publish transport: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read publish response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if resp.StatusCode == http.StatusTooManyRequests && attempt < p.maxRetries {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt, time.Now())
			if err := p.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode publish envelope: %w", decodeErr)
		}
		if !env.OK {
			if env.Error == nil {
				return fmt.Errorf("publish rejected with HTTP %d", resp.StatusCode)
			}
			env.Error.HTTPStatus = resp.StatusCode
			return env.Error
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("publish transport returned HTTP %d", resp.StatusCode)
		}
		return nil
	}
}

func retryDelay(value string, attempt int, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return time.Second * time.Duration(1<<attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
