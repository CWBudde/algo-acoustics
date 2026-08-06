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
	wait       time.Duration
	mu         sync.Mutex
	timer      debounceTimer
	cancelFn   context.CancelFunc
	generation uint64
	afterFunc  func(time.Duration, func()) debounceTimer
}

type debounceTimer interface {
	Stop() bool
}

func systemAfterFunc(wait time.Duration, fn func()) debounceTimer {
	return time.AfterFunc(wait, fn)
}

// NewDebouncer creates a debouncer with the given coalesce window.
func NewDebouncer(wait time.Duration) *Debouncer {
	return &Debouncer{wait: wait, afterFunc: systemAfterFunc}
}

// Trigger cancels any in-progress render and schedules renderFn to run
// after the debounce window expires. The context passed to renderFn is
// cancelled when a newer Trigger arrives or Cancel is called.
func (d *Debouncer) Trigger(renderFn func(ctx context.Context)) {
	d.mu.Lock()

	if d.cancelFn != nil {
		d.cancelFn()
	}

	if d.timer != nil {
		d.timer.Stop()
	}

	d.generation++
	generation := d.generation
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelFn = cancel

	afterFunc := d.afterFunc
	if afterFunc == nil {
		afterFunc = systemAfterFunc
	}

	d.timer = afterFunc(d.wait, func() {
		d.invoke(generation, ctx, cancel, renderFn)
	})
	d.mu.Unlock()
}

// Cancel stops the pending timer and cancels any in-progress render.
func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.generation++

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}

	if d.cancelFn != nil {
		d.cancelFn()
		d.cancelFn = nil
	}
}

func (d *Debouncer) invoke(generation uint64, ctx context.Context, cancel context.CancelFunc, renderFn func(context.Context)) {
	d.mu.Lock()
	if generation != d.generation {
		d.mu.Unlock()

		return
	}

	d.timer = nil
	d.mu.Unlock()

	defer func() {
		cancel()

		d.mu.Lock()
		if generation == d.generation {
			d.cancelFn = nil
		}
		d.mu.Unlock()
	}()

	renderFn(ctx)
}
