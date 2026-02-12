package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthCooldownEnabled(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		envSet   bool
		wantOn   bool
		wantDur  time.Duration
	}{
		{"unset", "", false, false, 0},
		{"empty", "", true, false, 0},
		{"true", "true", true, true, authCooldownDefaultDuration},
		{"1", "1", true, true, authCooldownDefaultDuration},
		{"yes", "yes", true, true, authCooldownDefaultDuration},
		{"on", "on", true, true, authCooldownDefaultDuration},
		{"false", "false", true, false, 0},
		{"0", "0", true, false, 0},
		{"no", "no", true, false, 0},
		{"off", "off", true, false, 0},
		{"30m", "30m", true, true, 30 * time.Minute},
		{"2h", "2h", true, true, 2 * time.Hour},
		{"garbage", "notaduration", true, false, 0},
		{"negative", "-5m", true, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv(authCooldownEnvVar, tt.envVal)
			} else {
				os.Unsetenv(authCooldownEnvVar)
			}

			dur, enabled := AuthCooldownEnabled()
			if enabled != tt.wantOn {
				t.Errorf("enabled = %v, want %v", enabled, tt.wantOn)
			}
			if dur != tt.wantDur {
				t.Errorf("duration = %v, want %v", dur, tt.wantDur)
			}
		})
	}
}

func TestTrySetAuthCooldown_FirstSetSucceeds(t *testing.T) {
	host := setupTestHost(t)

	ok, _, err := TrySetAuthCooldown(host, 8080, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected first set to succeed")
	}
}

func TestTrySetAuthCooldown_SecondSetBlocked(t *testing.T) {
	host := setupTestHost(t)

	ok, _, err := TrySetAuthCooldown(host, 8080, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error on first set: %v", err)
	}
	if !ok {
		t.Fatal("expected first set to succeed")
	}

	ok, _, err = TrySetAuthCooldown(host, 9090, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error on second set: %v", err)
	}
	if ok {
		t.Fatal("expected second set to be blocked by active cooldown")
	}
}

func TestTrySetAuthCooldown_StaleCooldownReplaced(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	// Create a stale cooldown file manually.
	cooldownPath, err := authCooldownPath(host)
	if err != nil {
		t.Fatalf("unexpected error getting cooldown path: %v", err)
	}
	staleInfo := authCooldownInfo{
		PID:       99999,
		Timestamp: time.Now().Add(-cooldownDuration - time.Second),
		Port:      7777,
	}
	data, _ := json.Marshal(staleInfo)
	if err := os.WriteFile(cooldownPath, data, 0o600); err != nil {
		t.Fatalf("failed to write stale cooldown: %v", err)
	}

	ok, _, err := TrySetAuthCooldown(host, 8080, cooldownDuration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected stale cooldown to be replaced")
	}

	// Verify the new cooldown has our PID.
	newData, _ := os.ReadFile(cooldownPath)
	var info authCooldownInfo
	json.Unmarshal(newData, &info)
	if info.PID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
}

func TestClearAuthCooldown_RemovesFile(t *testing.T) {
	host := setupTestHost(t)

	ok, _, err := TrySetAuthCooldown(host, 8080, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected set to succeed")
	}

	cooldownPath, _ := authCooldownPath(host)
	if _, err := os.Stat(cooldownPath); os.IsNotExist(err) {
		t.Fatal("cooldown file should exist before clear")
	}

	ClearAuthCooldown(host)

	if _, err := os.Stat(cooldownPath); !os.IsNotExist(err) {
		t.Fatal("cooldown file should not exist after clear")
	}
}

func TestTrySetAuthCooldown_DifferentHostsIndependent(t *testing.T) {
	host1 := setupTestHost(t)
	host2 := setupTestHostNamed(t, "other.example.com")

	ok1, _, err := TrySetAuthCooldown(host1, 8080, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok1 {
		t.Fatal("expected first host set to succeed")
	}

	ok2, _, err := TrySetAuthCooldown(host2, 9090, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok2 {
		t.Fatal("expected second host set to succeed (different host)")
	}
}

func TestClearAuthCooldown_AllowsReacquire(t *testing.T) {
	host := setupTestHost(t)

	ok, _, _ := TrySetAuthCooldown(host, 8080, 60*time.Minute)
	if !ok {
		t.Fatal("expected first set to succeed")
	}

	ClearAuthCooldown(host)

	ok, _, err := TrySetAuthCooldown(host, 9090, 60*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected re-acquire after clear to succeed")
	}
}

// setupTestHost redirects the cache dir to a temp directory so tests don't
// pollute the real filesystem.
func setupTestHost(t *testing.T) string {
	return setupTestHostNamed(t, "test.example.com")
}

func setupTestHostNamed(t *testing.T, hostname string) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(tmpDir, ".cache"))
	return "https://" + hostname
}
