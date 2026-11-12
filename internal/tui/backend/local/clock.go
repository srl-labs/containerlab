// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"time"
)

// clock is the source of time of the backend. Tests replace it to drive timers by hand.
type clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// sleep waits for d and reports whether it did so without ctx ending first.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// backoff yields the delays between the attempts to restart something that failed.
type backoff struct {
	first, limit time.Duration
	current      time.Duration
}

func newBackoff(first, limit time.Duration) *backoff {
	return &backoff{first: first, limit: limit}
}

// next returns the delay to wait before the next attempt and doubles it for the one after.
func (b *backoff) next() time.Duration {
	if b.current == 0 {
		b.current = b.first
	}

	d := b.current
	b.current = min(b.current*2, b.limit) //nolint:mnd // exponential growth

	return d
}

func (b *backoff) reset() { b.current = 0 }

// debounce collapses a burst of triggers into one action: the action is due once no trigger
// arrived for the quiet period, and at the latest a fixed time after the first trigger.
type debounce struct {
	quiet, limit time.Duration
	first, last  time.Time
	pending      bool
}

func (d *debounce) trigger(now time.Time) {
	if !d.pending {
		d.pending = true
		d.first = now
	}

	d.last = now
}

// due returns when the action has to run. It is only meaningful while a trigger is pending.
func (d *debounce) due() time.Time {
	quiet, limit := d.last.Add(d.quiet), d.first.Add(d.limit)
	if limit.Before(quiet) {
		return limit
	}

	return quiet
}

func (d *debounce) reset() { d.pending = false }
