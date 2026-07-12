package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestRateGateEnforcesGap(t *testing.T) {
	gate := newRateGate(100 * time.Millisecond)
	ctx := context.Background()

	start := time.Now()
	if err := gate.Wait(ctx); err != nil {
		t.Fatalf("first wait failed: %v", err)
	}
	if err := gate.Wait(ctx); err != nil {
		t.Fatalf("second wait failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("expected spacing between waits, got %v", elapsed)
	}
}

func TestRetryDelayUsesRetryAfter(t *testing.T) {
	got := retryDelay(0, 12*time.Second)
	if got != 12*time.Second {
		t.Fatalf("expected retry-after to be honored, got %v", got)
	}
}

func TestIsRetryableHTTPStatus(t *testing.T) {
	if !isRetryableHTTPStatus(http.StatusTooManyRequests) {
		t.Fatal("429 should be retryable")
	}
	if isRetryableHTTPStatus(http.StatusNotFound) {
		t.Fatal("404 should not be retryable")
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	header := http.Header{}
	header.Set("Retry-After", "3")
	if got := parseRetryAfterHeader(header); got != 3*time.Second {
		t.Fatalf("expected 3s retry-after, got %v", got)
	}
}
