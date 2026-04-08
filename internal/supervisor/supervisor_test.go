package supervisor

import (
	"os"
	"testing"
	"time"
)

func TestNewProcess(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())

	if p.cmd == nil {
		t.Fatal("cmd should not be nil")
	}
	if p.cmd.Dir != "/tmp" {
		t.Errorf("dir = %q, want /tmp", p.cmd.Dir)
	}
	if p.cmd.SysProcAttr == nil || !p.cmd.SysProcAttr.Setpgid {
		t.Error("Setpgid should be true")
	}
}

func TestStartAndWait(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())

	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if p.Pid() == 0 {
		t.Error("Pid should be non-zero after start")
	}

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if p.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", p.ExitCode())
	}
}

func TestGracefulStop(t *testing.T) {
	// sleep 60 will be killed by GracefulStop
	p := NewProcess("sleep", []string{"60"}, "/tmp", os.Environ())

	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	start := time.Now()
	p.GracefulStop(2)
	elapsed := time.Since(start)

	// Should complete quickly (SIGTERM kills sleep)
	if elapsed > 5*time.Second {
		t.Errorf("GracefulStop took %v, expected < 5s", elapsed)
	}
}

func TestKill(t *testing.T) {
	p := NewProcess("sleep", []string{"60"}, "/tmp", os.Environ())

	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := p.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}

	p.Wait() // Clean up zombie
}

func TestKillUnstartedProcess(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())

	// Should not panic or error
	if err := p.Kill(); err != nil {
		t.Errorf("Kill unstarted: %v", err)
	}
}

func TestPidBeforeStart(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())

	if p.Pid() != 0 {
		t.Errorf("Pid before start = %d, want 0", p.Pid())
	}
}

func TestWasSignaledDetectsSIGTERM(t *testing.T) {
	p := NewProcess("sleep", []string{"60"}, "/tmp", os.Environ())
	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// GracefulStop sends SIGTERM first
	_ = p.GracefulStop(5)

	// Go's ExitCode() returns -1 for signaled processes
	if p.ExitCode() != -1 {
		t.Errorf("ExitCode = %d, want -1 for signaled process", p.ExitCode())
	}

	signaled, sig := p.WasSignaled()
	if !signaled {
		t.Fatal("WasSignaled = false, want true after SIGTERM")
	}
	if sig != 15 {
		t.Errorf("signal = %d, want 15 (SIGTERM)", sig)
	}
}

func TestWasSignaledDetectsSIGKILL(t *testing.T) {
	p := NewProcess("sleep", []string{"60"}, "/tmp", os.Environ())
	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := p.Kill(); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	_ = p.Wait()

	signaled, sig := p.WasSignaled()
	if !signaled {
		t.Fatal("WasSignaled = false, want true after SIGKILL")
	}
	if sig != 9 {
		t.Errorf("signal = %d, want 9 (SIGKILL)", sig)
	}
}

func TestWasSignaledFalseOnNormalExit(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())
	if err := p.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	_ = p.Wait()

	signaled, _ := p.WasSignaled()
	if signaled {
		t.Error("WasSignaled = true, want false for normal exit")
	}
}

func TestWasSignaledBeforeStart(t *testing.T) {
	p := NewProcess("echo", []string{"hello"}, "/tmp", os.Environ())

	signaled, sig := p.WasSignaled()
	if signaled {
		t.Error("WasSignaled = true, want false before start")
	}
	if sig != 0 {
		t.Errorf("signal = %d, want 0 before start", sig)
	}
}
