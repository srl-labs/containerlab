// Copyright 2026 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package local

import (
	"context"
	"errors"
	"strings"
	"time"

	clabconstants "github.com/srl-labs/containerlab/constants"
	clabruntime "github.com/srl-labs/containerlab/runtime"
)

const (
	// A burst of runtime events, ie. a lab being deployed, causes one reload once the
	// events pause, and at least one per reloadMaxWait while they keep coming.
	reloadQuiet   = 250 * time.Millisecond
	reloadMaxWait = time.Second
	// reloadSafety is the interval of the reloads that happen regardless of events, to
	// catch up with whatever an event stream may have missed.
	reloadSafety = 30 * time.Second
	// pollInterval is the reload interval for runtimes that cannot stream events.
	pollInterval = 2 * time.Second

	streamBackoffFirst = time.Second
	streamBackoffLimit = 30 * time.Second
	// streamHealthy is how long an event stream has to last for its end to count as a new
	// failure rather than a repeated one.
	streamHealthy = time.Minute

	// execEventPrefix starts the actions of the events a command run in a container
	// causes. Health checks produce them all the time and they change nothing.
	execEventPrefix = "exec_"
)

// requestReload asks for a reload of the inventory soon. Requests in quick succession are
// served by one reload.
func (b *Backend) requestReload() {
	select {
	case b.reloadCh <- struct{}{}:
	default:
	}
}

// Refresh requests an immediate reload of the inventory and of the topology files.
func (b *Backend) Refresh() {
	select {
	case b.refreshCh <- struct{}{}:
	default:
	}
}

// watch keeps the inventory current until ctx ends: it reloads it when the runtime reported
// events, when the UI asks for it and at a slow pace in any case.
func (b *Backend) watch(ctx context.Context) {
	safety := time.NewTicker(reloadSafety)
	defer safety.Stop()

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	timer.Stop()

	pending := debounce{quiet: reloadQuiet, limit: reloadMaxWait}

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.reloadCh:
			now := time.Now()
			pending.trigger(now)
			timer.Reset(pending.due().Sub(now))

			continue
		case <-b.refreshCh:
			b.files.request()
			b.links.forget()
		case <-timer.C:
		case <-safety.C:
		}

		pending.reset()
		timer.Stop()
		b.reloadLogged(ctx)
	}
}

// reloadLogged reloads the inventory and reports a failure once, not on every attempt.
func (b *Backend) reloadLogged(ctx context.Context) {
	err := b.reload(ctx)

	switch {
	case ctx.Err() != nil:
	case err != nil && !b.reloadFailing:
		b.logf(levelError, "failed to list the %s: %v", b.listed(), err)
	case err == nil && b.reloadFailing:
		b.logf(levelInfo, "listing the %s works again", b.listed())
	}

	b.reloadFailing = err != nil
}

// streamEvents turns the events of the runtime into reload requests until ctx ends. The
// events only tell that something changed: what the state is now is always listed anew. A
// stream that ends is opened again, after a growing pause.
func (b *Backend) streamEvents(ctx context.Context) {
	pause := newBackoff(streamBackoffFirst, streamBackoffLimit)
	opts := clabruntime.EventStreamOptions{
		Labels: map[string]string{clabconstants.Containerlab: ""},
	}

	for ctx.Err() == nil {
		events, errs, err := b.rt.StreamEvents(ctx, opts)
		if err != nil {
			b.logf(levelInfo, "the %s runtime does not stream events, polling instead: %v",
				b.rtName, err)
			b.poll(ctx)

			return
		}

		// Whatever happened while no stream was open went unnoticed.
		b.requestReload()

		opened := time.Now()
		err = b.consumeEvents(ctx, events, errs)

		if ctx.Err() != nil {
			return
		}

		if time.Since(opened) > streamHealthy {
			pause.reset()
		}

		wait := pause.next()
		b.logf(levelWarn, "the runtime event stream ended, reconnecting in %s: %v", wait, err)

		if !sleep(ctx, wait) {
			return
		}
	}
}

// consumeEvents reads an event stream until it ends and returns why it did.
func (b *Backend) consumeEvents(
	ctx context.Context,
	events <-chan clabruntime.ContainerEvent,
	errs <-chan error,
) error {
	var streamErr error

	for events != nil || errs != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				events = nil

				continue
			}

			if isStateEvent(&ev) {
				b.requestReload()
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil

				continue
			}

			if err != nil {
				streamErr = err
			}
		}
	}

	if streamErr == nil {
		streamErr = errors.New("stream closed")
	}

	return streamErr
}

// isStateEvent reports whether an event can stand for a change of the inventory.
func isStateEvent(ev *clabruntime.ContainerEvent) bool {
	return !strings.HasPrefix(strings.ToLower(ev.Action), execEventPrefix)
}

// poll requests a reload at a fixed pace until ctx ends.
func (b *Backend) poll(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.requestReload()
		}
	}
}
