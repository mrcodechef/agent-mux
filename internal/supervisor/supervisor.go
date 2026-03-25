// Package supervisor handles process lifecycle: spawn with process group,
// signal handling, graceful shutdown, and artifact preservation.
package supervisor

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Process wraps an exec.Cmd with process group management.
type Process struct {
	cmd      *exec.Cmd
	pgid     int
	started  bool
	waitDone chan struct{} // closed when Wait completes
	waitErr  error
	waitOnce sync.Once
}

// NewProcess creates a supervised process with its own process group.
func NewProcess(ctx context.Context, name string, args []string, dir string, env []string) *Process {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return &Process{cmd: cmd, waitDone: make(chan struct{})}
}

// Cmd returns the underlying exec.Cmd for stdout/stderr pipe setup.
func (p *Process) Cmd() *exec.Cmd {
	return p.cmd
}

// Start starts the process.
func (p *Process) Start() error {
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w", err)
	}
	p.started = true

	// Get the process group ID
	if p.cmd.Process != nil {
		pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
		if err == nil {
			p.pgid = pgid
		} else {
			p.pgid = p.cmd.Process.Pid
		}
	}

	return nil
}

// Wait waits for the process to exit and returns the error.
// Safe to call multiple times -- only the first call actually waits.
func (p *Process) Wait() error {
	p.waitOnce.Do(func() {
		p.waitErr = p.cmd.Wait()
		close(p.waitDone)
	})
	<-p.waitDone
	return p.waitErr
}

// GracefulStop sends SIGTERM to the process group and waits for graceSec.
// If the process doesn't exit within graceSec, sends SIGKILL.
func (p *Process) GracefulStop(graceSec int) error {
	if !p.started || p.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM to process group
	if err := p.signalGroup(syscall.SIGTERM); err != nil {
		return fmt.Errorf("SIGTERM: %w", err)
	}

	// Wait for process to exit with grace period
	select {
	case <-p.waitDone:
		return p.waitErr
	default:
	}

	// Process still running, wait with timeout
	done := make(chan struct{})
	go func() {
		p.Wait()
		close(done)
	}()

	select {
	case <-done:
		return p.waitErr
	case <-time.After(time.Duration(graceSec) * time.Second):
		// Grace period expired, force kill
		p.signalGroup(syscall.SIGKILL)
		<-done
		return p.waitErr
	}
}

// Kill immediately kills the process group.
func (p *Process) Kill() error {
	if !p.started || p.cmd.Process == nil {
		return nil
	}
	return p.signalGroup(syscall.SIGKILL)
}

// signalGroup sends a signal to the entire process group.
func (p *Process) signalGroup(sig syscall.Signal) error {
	if p.pgid > 0 {
		return syscall.Kill(-p.pgid, sig)
	}
	if p.cmd.Process != nil {
		return p.cmd.Process.Signal(sig)
	}
	return nil
}

// Pid returns the process ID, or 0 if not started.
func (p *Process) Pid() int {
	if p.cmd.Process != nil {
		return p.cmd.Process.Pid
	}
	return 0
}

// ExitCode returns the exit code, or -1 if not available.
func (p *Process) ExitCode() int {
	if p.cmd.ProcessState != nil {
		return p.cmd.ProcessState.ExitCode()
	}
	return -1
}
