package liveness

import (
	"testing"
	"time"
)

func TestWatchdogInitialState(t *testing.T) {
	w := NewWatchdog(90, 180)

	action, silence := w.Check()
	if action != ActionNone {
		t.Errorf("initial action = %d, want ActionNone", action)
	}
	if silence > 1 {
		t.Errorf("initial silence = %d, should be ~0", silence)
	}
}

func TestWatchdogTouch(t *testing.T) {
	w := NewWatchdog(1, 2)

	// Force lastActivity to be old
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-3 * time.Second)
	w.mu.Unlock()

	// Should be in kill zone
	action, _ := w.Check()
	if action != ActionKill {
		t.Errorf("expected ActionKill, got %d", action)
	}

	// Reset via new watchdog (terminated flag was set)
	w2 := NewWatchdog(1, 2)
	w2.mu.Lock()
	w2.lastActivity = time.Now().Add(-3 * time.Second)
	w2.mu.Unlock()

	// Touch to reset
	w2.Touch()

	action, silence := w2.Check()
	if action != ActionNone {
		t.Errorf("after touch: action = %d, want ActionNone", action)
	}
	if silence > 1 {
		t.Errorf("after touch: silence = %d, should be ~0", silence)
	}
}

func TestWatchdogWarnThenKill(t *testing.T) {
	w := NewWatchdog(1, 3)

	// Move lastActivity back 2 seconds -> should warn
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-2 * time.Second)
	w.mu.Unlock()

	action, silence := w.Check()
	if action != ActionWarn {
		t.Errorf("expected ActionWarn at %ds silence, got %d", silence, action)
	}

	// Check again - warn already emitted, not yet kill
	action, _ = w.Check()
	if action != ActionNone {
		t.Errorf("expected ActionNone (warn already emitted), got %d", action)
	}

	// Move further back -> should kill
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-4 * time.Second)
	w.warnEmitted = true
	w.terminated = false
	w.mu.Unlock()

	action, silence = w.Check()
	if action != ActionKill {
		t.Errorf("expected ActionKill at %ds silence, got %d", silence, action)
	}
}

func TestWatchdogTerminate(t *testing.T) {
	w := NewWatchdog(1, 2)

	w.Terminate()

	if !w.IsTerminated() {
		t.Error("should be terminated")
	}

	// After termination, always returns ActionNone
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-10 * time.Second)
	w.mu.Unlock()

	action, _ := w.Check()
	if action != ActionNone {
		t.Errorf("after terminate: action = %d, want ActionNone", action)
	}
}

func TestWatchdogFirstTerminalWins(t *testing.T) {
	w := NewWatchdog(1, 2)

	// Move to kill zone
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-3 * time.Second)
	w.mu.Unlock()

	action1, _ := w.Check()
	if action1 != ActionKill {
		t.Errorf("first check: expected ActionKill, got %d", action1)
	}

	// Subsequent checks should return None (terminated)
	action2, _ := w.Check()
	if action2 != ActionNone {
		t.Errorf("second check: expected ActionNone (terminated), got %d", action2)
	}
}

func TestWatchdogTickerEmitsEvents(t *testing.T) {
	// Use large gap between warn and kill to avoid race
	w := NewWatchdog(1, 30)

	// Set activity 2 seconds ago (> warn=1s, << kill=30s)
	w.mu.Lock()
	w.lastActivity = time.Now().Add(-2 * time.Second)
	w.mu.Unlock()

	actions, stop := w.RunTicker(1)
	defer stop()

	select {
	case evt := <-actions:
		if evt.Action != ActionWarn {
			t.Errorf("expected ActionWarn, got %d", evt.Action)
		}
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for watchdog event")
	}
}
