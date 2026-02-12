package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	authCooldownEnvVar          = "DUPLO_JIT_AUTH_COOLDOWN"
	authCooldownDefaultDuration = 60 * time.Minute
)

type authCooldownInfo struct {
	PID       int       `json:"pid"`
	Timestamp time.Time `json:"timestamp"`
	Port      int       `json:"port"`
}

// AuthCooldownEnabled checks the DUPLO_JIT_AUTH_COOLDOWN environment variable.
// Returns the cooldown duration and whether cooldown is enabled.
//
// Values: "true"/"1" → 60m default, valid duration string (e.g. "30m") → parsed,
// unset/"false"/"0" → disabled.
func AuthCooldownEnabled() (time.Duration, bool) {
	val, ok := os.LookupEnv(authCooldownEnvVar)
	if !ok || val == "" {
		return 0, false
	}

	switch strings.ToLower(val) {
	case "false", "0", "no", "off":
		return 0, false
	case "true", "1", "yes", "on":
		return authCooldownDefaultDuration, true
	}

	d, err := time.ParseDuration(val)
	if err != nil || d <= 0 {
		return 0, false
	}
	return d, true
}

// authCooldownPath returns the path to the cooldown file for the given host URL.
// Cooldown files live in ~/.cache/duplo-jit-auth/{hostname}.cooldown so that
// duplo-jit and duplo-aws-credential-process can coordinate.
func authCooldownPath(host string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache dir: %w", err)
	}

	dir := filepath.Join(cacheDir, "duplo-jit-auth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create auth cooldown dir: %w", err)
	}

	hostname := GetHostCacheKey(host)
	return filepath.Join(dir, hostname+".cooldown"), nil
}

// TrySetAuthCooldown atomically creates a cooldown file for the given host.
// Returns (true, nil) if the cooldown was set (caller should open the browser),
// (false, nil) if a recent cooldown is already active (caller should NOT open the browser),
// or (false, err) on unexpected errors.
//
// Stale cooldowns (older than cooldownDuration) are automatically replaced.
func TrySetAuthCooldown(host string, port int, cooldownDuration time.Duration) (bool, time.Time, error) {
	cooldownPath, err := authCooldownPath(host)
	if err != nil {
		return false, time.Time{}, err
	}

	return trySetCooldown(cooldownPath, port, cooldownDuration, true)
}

func trySetCooldown(cooldownPath string, port int, cooldownDuration time.Duration, retryOnStale bool) (bool, time.Time, error) {
	info := authCooldownInfo{
		PID:       os.Getpid(),
		Timestamp: time.Now(),
		Port:      port,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to marshal cooldown info: %w", err)
	}

	f, err := os.OpenFile(cooldownPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return false, time.Time{}, fmt.Errorf("failed to create cooldown file: %w", err)
		}

		// Cooldown file exists — check if it's stale.
		if retryOnStale && isStale(cooldownPath, cooldownDuration) {
			os.Remove(cooldownPath)
			return trySetCooldown(cooldownPath, port, cooldownDuration, false)
		}

		expiry := readCooldownExpiry(cooldownPath, cooldownDuration)
		return false, expiry, nil
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(cooldownPath)
		return false, time.Time{}, fmt.Errorf("failed to write cooldown file: %w", err)
	}

	return true, time.Time{}, nil
}

func isStale(cooldownPath string, cooldownDuration time.Duration) bool {
	data, err := os.ReadFile(cooldownPath)
	if err != nil {
		return true // can't read → treat as stale
	}

	var info authCooldownInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return true // corrupt → treat as stale
	}

	return time.Since(info.Timestamp) > cooldownDuration
}

func readCooldownExpiry(cooldownPath string, cooldownDuration time.Duration) time.Time {
	data, err := os.ReadFile(cooldownPath)
	if err != nil {
		return time.Time{}
	}

	var info authCooldownInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return time.Time{}
	}

	return info.Timestamp.Add(cooldownDuration)
}

// ClearAuthCooldown removes the cooldown file for the given host.
// Called after successful authentication. No-op if the file doesn't exist.
func ClearAuthCooldown(host string) {
	cooldownPath, err := authCooldownPath(host)
	if err != nil {
		return
	}
	os.Remove(cooldownPath)
}
