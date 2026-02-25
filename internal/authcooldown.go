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
	Admin     bool      `json:"admin"`
}

// IsAuthCooldownEnabled checks the DUPLO_JIT_AUTH_COOLDOWN environment variable.
// Returns the cooldown duration and whether cooldown is enabled.
//
// Values: "true"/"1" → 60m default, valid duration string (e.g. "30m") → parsed,
// unset/"false"/"0" → disabled.
func IsAuthCooldownEnabled() (time.Duration, bool) {
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

// authCooldownPath returns the path to the cooldown file for the given host URL
// and admin flag. Cooldown files live in ~/.cache/duplo-jit-auth/.
func authCooldownPath(host string, admin bool) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache dir: %w", err)
	}

	dir := filepath.Join(cacheDir, "duplo-jit-auth")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create auth cooldown dir: %w", err)
	}

	hostname := GetHostCacheKey(host)
	suffix := ".cooldown"
	if admin {
		suffix = ".admin.cooldown"
	}
	return filepath.Join(dir, hostname+suffix), nil
}

// TrySetAuthCooldown atomically creates a cooldown file for the given host and admin flag.
// Returns (true, zero, nil) if the cooldown was set (caller should open the browser),
// (false, expiry, nil) if a recent cooldown is already active,
// or (false, zero, err) on unexpected errors.
//
// Stale cooldowns (older than cooldownDuration) are automatically replaced.
func TrySetAuthCooldown(host string, port int, admin bool, cooldownDuration time.Duration) (bool, time.Time, error) {
	cooldownPath, err := authCooldownPath(host, admin)
	if err != nil {
		return false, time.Time{}, err
	}

	return trySetCooldown(cooldownPath, port, admin, cooldownDuration, true)
}

func trySetCooldown(cooldownPath string, port int, admin bool, cooldownDuration time.Duration, retryOnStale bool) (bool, time.Time, error) {
	info := authCooldownInfo{
		PID:       os.Getpid(),
		Timestamp: time.Now(),
		Port:      port,
		Admin:     admin,
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
		existing := readInfoFromPath(cooldownPath)
		if retryOnStale && (existing == nil || time.Since(existing.Timestamp) > cooldownDuration) {
			// Atomically move stale file out of the way to avoid TOCTOU race.
			// Other processes will find the file already gone and fall through to O_EXCL create.
			tmpPath := fmt.Sprintf("%s.stale.%d", cooldownPath, os.Getpid())
			if os.Rename(cooldownPath, tmpPath) == nil {
				_ = os.Remove(tmpPath)
			}
			return trySetCooldown(cooldownPath, port, admin, cooldownDuration, false)
		}

		var expiry time.Time
		if existing != nil {
			expiry = existing.Timestamp.Add(cooldownDuration)
		}
		return false, expiry, nil
	}
	defer f.Close() //nolint:errcheck // best-effort close on deferred cleanup

	if _, err := f.Write(data); err != nil {
		_ = os.Remove(cooldownPath)
		return false, time.Time{}, fmt.Errorf("failed to write cooldown file: %w", err)
	}

	return true, time.Time{}, nil
}

func readInfoFromPath(path string) *authCooldownInfo {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var info authCooldownInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil
	}
	return &info
}

// ReadCooldownInfo reads the cooldown file for the given host and admin flag.
// Returns nil if no cooldown file exists or it cannot be read.
func ReadCooldownInfo(host string, admin bool) *authCooldownInfo {
	cooldownPath, err := authCooldownPath(host, admin)
	if err != nil {
		return nil
	}
	return readInfoFromPath(cooldownPath)
}

// UpdateCooldown rewrites the cooldown file with the current PID and the given port,
// preserving the original timestamp. The timestamp represents when the browser tab
// was opened and should only be reset when a new tab is actually opened (via
// TrySetAuthCooldown). Used when a relay process takes over a dead process's port.
func UpdateCooldown(host string, admin bool, port int) error {
	cooldownPath, err := authCooldownPath(host, admin)
	if err != nil {
		return err
	}

	// Read existing timestamp from the cooldown file.
	existing := ReadCooldownInfo(host, admin)
	ts := time.Now()
	if existing != nil {
		ts = existing.Timestamp
	}

	info := authCooldownInfo{
		PID:       os.Getpid(),
		Timestamp: ts,
		Port:      port,
		Admin:     admin,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(cooldownPath, data, 0o600)
}

// ClearAuthCooldown removes the cooldown file for the given host and admin flag.
// Called after successful authentication. No-op if the file doesn't exist.
func ClearAuthCooldown(host string, admin bool) {
	cooldownPath, err := authCooldownPath(host, admin)
	if err != nil {
		return
	}
	_ = os.Remove(cooldownPath)
}
