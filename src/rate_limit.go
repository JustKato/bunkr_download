package main

import (
	"context"
	"sync"
	"time"
)

const (
	apiRequestGap      = 400 * time.Millisecond
	downloadRequestGap = 800 * time.Millisecond
)

type rateGate struct {
	mu     sync.Mutex
	gap    time.Duration
	nextAt time.Time
}

func newRateGate(gap time.Duration) *rateGate {
	return &rateGate{gap: gap}
}

func (g *rateGate) Wait(ctx context.Context) error {
	if g == nil || g.gap <= 0 {
		return nil
	}

	g.mu.Lock()
	wait := time.Until(g.nextAt)
	if wait <= 0 {
		g.nextAt = time.Now().Add(g.gap)
		g.mu.Unlock()
		return nil
	}
	g.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}

	g.mu.Lock()
	g.nextAt = time.Now().Add(g.gap)
	g.mu.Unlock()
	return nil
}
