package config

import (
	"os"
	"testing"
)

func TestTimeoutForEffort(t *testing.T) {
	tests := []struct {
		effort string
		want   int
	}{
		{"low", 60},
		{"medium", 300},
		{"high", 900},
		{"xhigh", 1800},
		{"unknown", 900}, // defaults to high
		{"HIGH", 900},    // case insensitive
		{"  high  ", 900},
	}

	for _, tt := range tests {
		got := TimeoutForEffort(tt.effort)
		if got != tt.want {
			t.Errorf("TimeoutForEffort(%q) = %d, want %d", tt.effort, got, tt.want)
		}
	}
}

func TestGraceSec(t *testing.T) {
	if got := GraceSec(); got != 60 {
		t.Fatalf("GraceSec() = %d, want 60", got)
	}
}

func TestMaxDepthDefault(t *testing.T) {
	t.Setenv("AGENT_MUX_MAX_DEPTH", "")
	if got := MaxDepth(); got != 2 {
		t.Fatalf("MaxDepth() = %d, want 2", got)
	}
}

func TestMaxDepthEnv(t *testing.T) {
	t.Setenv("AGENT_MUX_MAX_DEPTH", "5")
	if got := MaxDepth(); got != 5 {
		t.Fatalf("MaxDepth() = %d, want 5", got)
	}
}

func TestMaxDepthInvalidEnv(t *testing.T) {
	t.Setenv("AGENT_MUX_MAX_DEPTH", "not-a-number")
	if got := MaxDepth(); got != 2 {
		t.Fatalf("MaxDepth(invalid) = %d, want default 2", got)
	}
}

func TestPermissionModeDefault(t *testing.T) {
	t.Setenv("AGENT_MUX_PERMISSION_MODE", "")
	if got := PermissionMode(); got != "" {
		t.Fatalf("PermissionMode() = %q, want empty", got)
	}
}

func TestPermissionModeEnv(t *testing.T) {
	t.Setenv("AGENT_MUX_PERMISSION_MODE", "default")
	if got := PermissionMode(); got != "default" {
		t.Fatalf("PermissionMode() = %q, want %q", got, "default")
	}
}

func TestHeartbeatIntervalSecDefault(t *testing.T) {
	t.Setenv("AGENT_MUX_HEARTBEAT_INTERVAL_SEC", "")
	if got := HeartbeatIntervalSec(); got != 15 {
		t.Fatalf("HeartbeatIntervalSec() = %d, want 15", got)
	}
}

func TestHeartbeatIntervalSecEnv(t *testing.T) {
	t.Setenv("AGENT_MUX_HEARTBEAT_INTERVAL_SEC", "30")
	if got := HeartbeatIntervalSec(); got != 30 {
		t.Fatalf("HeartbeatIntervalSec() = %d, want 30", got)
	}
}

func TestSilenceWarnSecondsDefault(t *testing.T) {
	t.Setenv("AGENT_MUX_SILENCE_WARN_SECONDS", "")
	if got := SilenceWarnSeconds(); got != 90 {
		t.Fatalf("SilenceWarnSeconds() = %d, want 90", got)
	}
}

func TestSilenceKillSecondsDefault(t *testing.T) {
	t.Setenv("AGENT_MUX_SILENCE_KILL_SECONDS", "")
	if got := SilenceKillSeconds(); got != 180 {
		t.Fatalf("SilenceKillSeconds() = %d, want 180", got)
	}
}

func TestDefaultModels(t *testing.T) {
	models := DefaultModels()
	if len(models["codex"]) == 0 {
		t.Fatal("DefaultModels() missing codex models")
	}
	if len(models["claude"]) == 0 {
		t.Fatal("DefaultModels() missing claude models")
	}
	if len(models["gemini"]) == 0 {
		t.Fatal("DefaultModels() missing gemini models")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Field: "timeout", Source: "test.md", Value: -1}
	if !IsValidationError(err) {
		t.Fatal("IsValidationError should return true")
	}
	if err.Error() == "" {
		t.Fatal("ValidationError.Error() should not be empty")
	}
}

func TestDeduplicateStrings(t *testing.T) {
	got := deduplicateStrings([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("deduplicateStrings = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("deduplicateStrings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeduplicateStringsNil(t *testing.T) {
	got := deduplicateStrings(nil)
	if got != nil {
		t.Fatalf("deduplicateStrings(nil) = %v, want nil", got)
	}
}

func TestEnvInt(t *testing.T) {
	key := "AGENT_MUX_TEST_INT_" + t.Name()
	defer os.Unsetenv(key)

	// Unset -> default
	if got := envInt(key, 42); got != 42 {
		t.Fatalf("envInt(unset) = %d, want 42", got)
	}

	// Valid
	os.Setenv(key, "10")
	if got := envInt(key, 42); got != 10 {
		t.Fatalf("envInt(10) = %d, want 10", got)
	}

	// Invalid -> default
	os.Setenv(key, "abc")
	if got := envInt(key, 42); got != 42 {
		t.Fatalf("envInt(abc) = %d, want 42", got)
	}

	// Zero -> default (must be > 0)
	os.Setenv(key, "0")
	if got := envInt(key, 42); got != 42 {
		t.Fatalf("envInt(0) = %d, want 42", got)
	}
}
