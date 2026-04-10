package algoacoustics

import (
	"context"
	"sync"
	"time"
)

// Debouncer coalesces rapid parameter changes within a time window.
// Each Trigger call cancels any in-progress render and resets the timer.
// When the timer fires, the render function runs with a fresh context.
type Debouncer struct {
	wait     time.Duration
	mu       sync.Mutex
	timer    *time.Timer
	cancelFn context.CancelFunc
}

// NewDebouncer creates a debouncer with the given coalesce window.
func NewDebouncer(wait time.Duration) *Debouncer {
	return &Debouncer{wait: wait}
}

// Trigger cancels any in-progress render and schedules renderFn to run
// after the debounce window expires. The context passed to renderFn is
// cancelled when a newer Trigger arrives or Cancel is called.
func (d *Debouncer) Trigger(renderFn func(ctx context.Context)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancelFn != nil {
		d.cancelFn()
	}

	if d.timer != nil {
		d.timer.Stop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelFn = cancel

	d.timer = time.AfterFunc(d.wait, func() {
		renderFn(ctx)
	})
}

// Cancel stops the pending timer and cancels any in-progress render.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	if d.cancelFn != nil {
		d.cancelFn()
		d.cancelFn = nil
	}
}
