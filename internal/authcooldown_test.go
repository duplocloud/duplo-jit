package internal

import (
	"os"
	"testing"
	"time"
)

func TestIsAuthCooldownEnabled(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		envSet  bool
		wantOn  bool
		wantDur time.Duration
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
				_ = os.Unsetenv(authCooldownEnvVar)
			}

			dur, enabled := IsAuthCooldownEnabled()
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
	mustSetCooldown(t, host, 8080, false, 60*time.Minute)
}

func TestTrySetAuthCooldown_SecondSetBlocked(t *testing.T) {
	host := setupTestHost(t)
	mustSetCooldown(t, host, 8080, false, 60*time.Minute)

	ok, _, err := TrySetAuthCooldown(host, 9090, false, 60*time.Minute)
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
	writeFakeCooldown(t, host, false, 99999, 7777, time.Now().Add(-cooldownDuration-time.Second))

	ok, _, err := TrySetAuthCooldown(host, 8080, false, cooldownDuration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected stale cooldown to be replaced")
	}

	// Verify the new cooldown has our PID.
	info := ReadCooldownInfo(host, false)
	if info.PID != os.Getpid() {
		t.Fatalf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
}

func TestClearAuthCooldown_RemovesFile(t *testing.T) {
	host := setupTestHost(t)
	mustSetCooldown(t, host, 8080, false, 60*time.Minute)

	cooldownPath, _ := authCooldownPath(host, false)
	if _, err := os.Stat(cooldownPath); os.IsNotExist(err) {
		t.Fatal("cooldown file should exist before clear")
	}

	ClearAuthCooldown(host, false)

	if _, err := os.Stat(cooldownPath); !os.IsNotExist(err) {
		t.Fatal("cooldown file should not exist after clear")
	}
}

func TestTrySetAuthCooldown_DifferentHostsIndependent(t *testing.T) {
	host1 := setupTestHost(t)
	host2 := setupTestHostNamed(t, "other.example.com")

	mustSetCooldown(t, host1, 8080, false, 60*time.Minute)
	mustSetCooldown(t, host2, 9090, false, 60*time.Minute)
}

func TestClearAuthCooldown_AllowsReacquire(t *testing.T) {
	host := setupTestHost(t)
	mustSetCooldown(t, host, 8080, false, 60*time.Minute)
	ClearAuthCooldown(host, false)
	mustSetCooldown(t, host, 9090, false, 60*time.Minute)
}

func TestTrySetAuthCooldown_AdminFlagIndependent(t *testing.T) {
	host := setupTestHost(t)
	mustSetCooldown(t, host, 8080, false, 60*time.Minute)
	mustSetCooldown(t, host, 9090, true, 60*time.Minute)
}
