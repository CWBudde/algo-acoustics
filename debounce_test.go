package algoacoustics

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDebouncer_CoalescesRapidCalls(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(50 * time.Millisecond)
	defer d.Cancel()

	var callCount atomic.Int32

	for range 10 {
		d.Trigger(func(_ context.Context) {
			callCount.Add(1)
		})

		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the debounce window to fire after the last trigger.
	time.Sleep(100 * time.Millisecond)

	if got := callCount.Load(); got != 1 {
		t.Errorf("callCount = %d, want 1 (debounce should coalesce)", got)
	}
}

func TestDebouncer_Cancel(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(50 * time.Millisecond)

	var called atomic.Bool

	d.Trigger(func(_ context.Context) {
		called.Store(true)
	})

	d.Cancel()
	time.Sleep(100 * time.Millisecond)

	if called.Load() {
		t.Error("render function called after Cancel")
	}
}

func TestDebouncer_CancelsContext(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(20 * time.Millisecond)
	defer d.Cancel()

	var mu sync.Mutex
	var ctxErrors []error

	// First trigger — will be cancelled by the second.
	d.Trigger(func(ctx context.Context) {
		// By the time this runs (if it runs), context should be cancelled
		// because we trigger again before the window fires.
		mu.Lock()

		ctxErrors = append(ctxErrors, ctx.Err())
		mu.Unlock()
	})

	// Immediately trigger again — cancels the first context.
	d.Trigger(func(ctx context.Context) {
		mu.Lock()

		ctxErrors = append(ctxErrors, ctx.Err())
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Only the second trigger should fire, with a non-cancelled context.
	if len(ctxErrors) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(ctxErrors))
	}

	if ctxErrors[0] != nil {
		t.Errorf("second trigger's context error = %v, want nil", ctxErrors[0])
	}
}

func TestDebouncer_MultipleSequentialTriggers(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(30 * time.Millisecond)
	defer d.Cancel()

	var callCount atomic.Int32

	// First burst
	d.Trigger(func(_ context.Context) { callCount.Add(1) })
	time.Sleep(60 * time.Millisecond) // Wait for first to fire

	// Second burst
	d.Trigger(func(_ context.Context) { callCount.Add(1) })
	time.Sleep(60 * time.Millisecond) // Wait for second to fire

	if got := callCount.Load(); got != 2 {
		t.Errorf("callCount = %d, want 2 (two separated bursts)", got)
	}
}
