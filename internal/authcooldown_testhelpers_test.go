package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

// writeFakeCooldown creates a cooldown file with the given parameters for testing.
func writeFakeCooldown(t *testing.T, host string, admin bool, pid int, port int, timestamp time.Time) {
	t.Helper()
	cooldownPath, err := authCooldownPath(host, admin)
	if err != nil {
		t.Fatalf("unexpected error getting cooldown path: %v", err)
	}
	info := authCooldownInfo{
		PID:       pid,
		Timestamp: timestamp,
		Port:      port,
		Admin:     admin,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal cooldown info: %v", err)
	}
	if err := os.WriteFile(cooldownPath, data, 0o600); err != nil {
		t.Fatalf("failed to write cooldown file: %v", err)
	}
}

// mustSetCooldown calls TrySetAuthCooldown and fails the test if it doesn't succeed.
func mustSetCooldown(t *testing.T, host string, port int, admin bool, duration time.Duration) {
	t.Helper()
	ok, _, err := TrySetAuthCooldown(host, port, admin, duration)
	if err != nil {
		t.Fatalf("unexpected error setting cooldown: %v", err)
	}
	if !ok {
		t.Fatal("expected cooldown set to succeed")
	}
}
