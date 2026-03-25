// Package liveness implements the watchdog that detects frozen harnesses
// from event stream silence.
package liveness

import (
	"sync"
	"time"
)


// Action represents what the watchdog recommends.
type Action int

const (
	ActionNone    Action = iota
	ActionWarn           // Silence exceeded warn threshold
	ActionKill           // Silence exceeded kill threshold
)

// Watchdog monitors harness event stream activity.
// It tracks the timestamp of the last harness event and signals
// when silence exceeds configured thresholds.
type Watchdog struct {
	mu            sync.Mutex
	lastActivity  time.Time
	warnSeconds   int
	killSeconds   int
	warnEmitted   bool
	terminated    bool
}

// NewWatchdog creates a liveness watchdog with the given thresholds.
func NewWatchdog(silenceWarnSec, silenceKillSec int) *Watchdog {
	return &Watchdog{
		lastActivity: time.Now(),
		warnSeconds:  silenceWarnSec,
		killSeconds:  silenceKillSec,
	}
}

// Touch updates the last activity timestamp. Called whenever
// a harness event is received from the event stream.
func (w *Watchdog) Touch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastActivity = time.Now()
	w.warnEmitted = false // Reset warn if we get new activity
}

// Check returns the recommended action based on silence duration.
// Returns (action, silenceSeconds).
func (w *Watchdog) Check() (Action, int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.terminated {
		return ActionNone, 0
	}

	silence := int(time.Since(w.lastActivity).Seconds())

	if silence >= w.killSeconds {
		w.terminated = true
		return ActionKill, silence
	}

	if silence >= w.warnSeconds && !w.warnEmitted {
		w.warnEmitted = true
		return ActionWarn, silence
	}

	return ActionNone, silence
}

// Terminate marks the watchdog as done (dispatch reached terminal state).
func (w *Watchdog) Terminate() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.terminated = true
}

// IsTerminated returns whether the watchdog has been terminated.
func (w *Watchdog) IsTerminated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.terminated
}

// SilenceDuration returns the current silence duration in seconds.
func (w *Watchdog) SilenceDuration() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return int(time.Since(w.lastActivity).Seconds())
}

// RunTicker starts a goroutine that checks liveness at regular intervals.
// Returns a channel that receives actions and a stop function.
func (w *Watchdog) RunTicker(checkIntervalSec int) (actions <-chan WatchdogEvent, stop func()) {
	ch := make(chan WatchdogEvent, 10)
	done := make(chan struct{})

	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Duration(checkIntervalSec) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				action, silence := w.Check()
				if action != ActionNone {
					ch <- WatchdogEvent{Action: action, SilenceSeconds: silence}
				}
			case <-done:
				return
			}
		}
	}()

	var stopOnce sync.Once
	return ch, func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
}

// WatchdogEvent is emitted when the watchdog detects silence.
type WatchdogEvent struct {
	Action         Action
	SilenceSeconds int
}
