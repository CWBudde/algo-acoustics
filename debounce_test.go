package algoacoustics

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

const debouncerTestDeadline = 2 * time.Second

type controlledDebounceTimer struct {
	mu      sync.Mutex
	fn      func()
	stopped bool
	fired   bool
}

func (timer *controlledDebounceTimer) Stop() bool {
	timer.mu.Lock()
	defer timer.mu.Unlock()

	active := !timer.stopped && !timer.fired
	timer.stopped = true

	return active
}

// Fire invokes even a stopped timer to model time.AfterFunc's documented race:
// Stop may return false because the callback has already been dispatched.
func (timer *controlledDebounceTimer) Fire() {
	timer.mu.Lock()
	if timer.fired {
		timer.mu.Unlock()

		return
	}

	timer.fired = true
	fn := timer.fn
	timer.mu.Unlock()

	fn()
}

type controlledDebounceScheduler struct {
	mu     sync.Mutex
	timers []*controlledDebounceTimer
}

func (scheduler *controlledDebounceScheduler) AfterFunc(_ time.Duration, fn func()) debounceTimer {
	timer := &controlledDebounceTimer{fn: fn}

	scheduler.mu.Lock()
	scheduler.timers = append(scheduler.timers, timer)
	scheduler.mu.Unlock()

	return timer
}

func (scheduler *controlledDebounceScheduler) Timers(t *testing.T) []*controlledDebounceTimer {
	t.Helper()

	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()

	return append([]*controlledDebounceTimer(nil), scheduler.timers...)
}

func newControlledDebouncer() (*Debouncer, *controlledDebounceScheduler) {
	scheduler := &controlledDebounceScheduler{}

	return &Debouncer{wait: time.Hour, afterFunc: scheduler.AfterFunc}, scheduler
}

func receiveDebouncerValue[T any](t *testing.T, values <-chan T) T {
	t.Helper()

	select {
	case value := <-values:
		return value
	case <-time.After(debouncerTestDeadline):
		t.Fatal("timed out waiting for debouncer callback")

		var zero T

		return zero
	}
}

func TestDebouncer_CoalescesRapidCalls(t *testing.T) {
	t.Parallel()

	d, scheduler := newControlledDebouncer()
	defer d.Cancel()

	called := make(chan int, 10)

	for index := range 10 {
		d.Trigger(func(context.Context) {
			called <- index
		})
	}

	timers := scheduler.Timers(t)
	if len(timers) != 10 {
		t.Fatalf("scheduled timers = %d, want 10", len(timers))
	}

	// Force every stopped timer closure to run, as if Stop lost a dispatch
	// race. Only the current generation may invoke its render function.
	for _, timer := range timers {
		timer.Fire()
	}

	if got := receiveDebouncerValue(t, called); got != 9 {
		t.Fatalf("callback index = %d, want final trigger 9", got)
	}

	select {
	case got := <-called:
		t.Fatalf("stale timer invoked callback %d", got)
	default:
	}
}

func TestDebouncer_CancelInvalidatesDispatchedTimer(t *testing.T) {
	t.Parallel()

	d, scheduler := newControlledDebouncer()
	called := make(chan struct{}, 1)

	d.Trigger(func(context.Context) {
		called <- struct{}{}
	})
	d.Cancel()

	timers := scheduler.Timers(t)
	if len(timers) != 1 {
		t.Fatalf("scheduled timers = %d, want 1", len(timers))
	}

	timers[0].Fire()

	select {
	case <-called:
		t.Fatal("render function called after Cancel")
	default:
	}
}

func TestDebouncer_NewerTriggerCancelsStartedContext(t *testing.T) {
	t.Parallel()

	d, scheduler := newControlledDebouncer()
	defer d.Cancel()

	started := make(chan struct{})
	canceled := make(chan error, 1)
	firstDone := make(chan struct{})

	d.Trigger(func(ctx context.Context) {
		close(started)
		<-ctx.Done()

		canceled <- ctx.Err()
	})

	firstTimer := scheduler.Timers(t)[0]
	go func() {
		firstTimer.Fire()
		close(firstDone)
	}()

	receiveDebouncerValue(t, started)

	secondCalled := make(chan error, 1)

	d.Trigger(func(ctx context.Context) {
		secondCalled <- ctx.Err()
	})

	err := receiveDebouncerValue(t, canceled)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("started callback context error = %v, want context.Canceled", err)
	}

	receiveDebouncerValue(t, firstDone)

	timers := scheduler.Timers(t)
	if len(timers) != 2 {
		t.Fatalf("scheduled timers = %d, want 2", len(timers))
	}

	timers[1].Fire()

	err = receiveDebouncerValue(t, secondCalled)
	if err != nil {
		t.Fatalf("current callback context error = %v, want nil", err)
	}
}

func TestDebouncer_MultipleSequentialTriggers(t *testing.T) {
	t.Parallel()

	d, scheduler := newControlledDebouncer()
	defer d.Cancel()

	called := make(chan int, 2)

	d.Trigger(func(context.Context) { called <- 1 })
	scheduler.Timers(t)[0].Fire()

	d.Trigger(func(context.Context) { called <- 2 })
	scheduler.Timers(t)[1].Fire()

	if got := receiveDebouncerValue(t, called); got != 1 {
		t.Fatalf("first callback = %d, want 1", got)
	}

	if got := receiveDebouncerValue(t, called); got != 2 {
		t.Fatalf("second callback = %d, want 2", got)
	}
}

func TestDebouncer_DefaultTimerFires(t *testing.T) {
	t.Parallel()

	d := NewDebouncer(time.Millisecond)
	defer d.Cancel()

	called := make(chan struct{})

	d.Trigger(func(context.Context) {
		close(called)
	})

	receiveDebouncerValue(t, called)
}
