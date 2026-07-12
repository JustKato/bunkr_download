package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPRetries = 5
	retryBaseDelay     = 2 * time.Second
	maxRetryDelay      = 45 * time.Second
)

func isRetryableHTTPStatus(code int) bool {
	return code == http.StatusTooManyRequests ||
		code == http.StatusRequestTimeout ||
		code >= http.StatusInternalServerError
}

func parseRetryAfterHeader(header http.Header) time.Duration {
	raw := strings.TrimSpace(header.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		if wait := time.Until(when); wait > 0 {
			return wait
		}
	}
	return 0
}

func retryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > maxRetryDelay {
			return maxRetryDelay
		}
		return retryAfter
	}

	delay := retryBaseDelay * time.Duration(1<<attempt)
	if delay > maxRetryDelay {
		return maxRetryDelay
	}
	return delay
}

func (s *BunkrService) doRequestWithRetry(
	ctx context.Context,
	client *http.Client,
	gate *rateGate,
	build func() (*http.Request, error),
) (*http.Response, error) {
	if client == nil {
		client = s.client
	}
	var lastErr error

	maxRetries := s.maxHTTPRetries()
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := gate.Wait(ctx); err != nil {
			return nil, err
		}

		req, err := build()
		if err != nil {
			return nil, err
		}
		req = req.WithContext(ctx)

		response, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt == maxRetries || ctx.Err() != nil {
				return nil, err
			}
			if err := sleepWithContext(ctx, retryDelay(attempt, 0)); err != nil {
				return nil, err
			}
			continue
		}

		if !isRetryableHTTPStatus(response.StatusCode) || attempt == maxRetries {
			return response, nil
		}

		retryAfter := parseRetryAfterHeader(response.Header)
		lastErr = fmt.Errorf("HTTP %s", response.Status)
		response.Body.Close()

		if err := sleepWithContext(ctx, retryDelay(attempt, retryAfter)); err != nil {
			return nil, err
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("request failed after retries")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
