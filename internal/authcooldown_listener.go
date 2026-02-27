package internal

import (
	"fmt"
	"log"
	"net"
	"time"
)

// checkCooldownBeforeListen evaluates existing cooldown state and decides how to proceed.
// Returns the port to listen on, whether to open a browser, the adjusted timeout, and
// an optional early result (non-nil means the caller should return immediately).
func checkCooldownBeforeListen(baseUrl string, admin bool, cmd string, defaultPort int, cooldownDuration time.Duration) (listenPort int, openBrowser bool, timeout time.Duration, earlyResult *TokenResult) {
	listenPort = defaultPort
	openBrowser = true
	timeout = cooldownDuration

	info := ReadCooldownInfo(baseUrl, admin)
	if info == nil {
		return
	}

	remaining := cooldownDuration - time.Since(info.Timestamp)
	if remaining <= 0 {
		ClearAuthCooldown(baseUrl, admin)
		return
	}

	if IsPidAlive(info.PID) {
		result := waitForCooldownHolder(baseUrl, admin, cmd, defaultPort, info, cooldownDuration)
		return defaultPort, true, cooldownDuration, &result
	}

	log.Printf("auth cooldown: previous process (PID %d) is dead, attempting relay on port %d", info.PID, info.Port)
	return info.Port, false, remaining, nil
}

// recoverRelayBindFailure handles the case where binding the relay port failed.
// Re-checks whether another relay process took over, or resets for a fresh start.
func recoverRelayBindFailure(baseUrl string, admin bool, cmd string, defaultPort int, relayPort int, cooldownDuration time.Duration) TokenResult {
	info := ReadCooldownInfo(baseUrl, admin)
	if info != nil && IsPidAlive(info.PID) {
		return waitForCooldownHolder(baseUrl, admin, cmd, defaultPort, info, cooldownDuration)
	}
	log.Printf("auth cooldown: port %d unavailable, resetting cooldown", relayPort)
	ClearAuthCooldown(baseUrl, admin)
	if result := cachedTokenResult(baseUrl); result != nil {
		return *result
	}
	return TokenViaListener(baseUrl, admin, cmd, defaultPort, 180*time.Second)
}

// acquireOrUpdateCooldown atomically sets a new cooldown (fresh start) or updates
// an existing one (relay). Returns non-nil if the caller should return early.
func acquireOrUpdateCooldown(baseUrl string, admin bool, cmd string, defaultPort int, localPort int, openBrowser bool, cooldownDuration time.Duration, listener net.Listener) *TokenResult {
	if openBrowser {
		acquired, expiry, cooldownErr := TrySetAuthCooldown(baseUrl, localPort, admin, cooldownDuration)
		if cooldownErr != nil {
			_ = listener.Close()
			result := TokenResult{err: fmt.Errorf("auth cooldown error: %w", cooldownErr)}
			return &result
		}
		if !acquired {
			_ = listener.Close()
			info := ReadCooldownInfo(baseUrl, admin)
			if info != nil {
				result := waitForCooldownHolder(baseUrl, admin, cmd, defaultPort, info, cooldownDuration)
				return &result
			}
			result := TokenResult{err: fmt.Errorf(
				"authentication for %s was recently attempted (expires %s)\n"+
					"To force a new attempt: duplo-jit clear-cache  (or unset %s to disable cooldown)",
				GetHostCacheKey(baseUrl), expiry.Format(time.RFC3339), authCooldownEnvVar)}
			return &result
		}
	} else {
		if err := UpdateCooldown(baseUrl, admin, localPort); err != nil {
			log.Printf("auth cooldown: failed to update cooldown for relay: %v", err)
		}
	}
	return nil
}

// cachedTokenResult returns a TokenResult from the credential cache, or nil if unavailable.
func cachedTokenResult(baseUrl string) *TokenResult {
	token := CacheGetDuploTokenUnchecked(baseUrl)
	if token == "" {
		return nil
	}
	log.Printf("auth cooldown: using cached credentials from completed auth process for %s", GetHostCacheKey(baseUrl))
	return &TokenResult{Token: token}
}

// waitForCooldownHolder waits for the active cooldown holder to finish, then retries.
func waitForCooldownHolder(baseUrl string, admin bool, cmd string, defaultPort int, info *authCooldownInfo, cooldownDuration time.Duration) TokenResult {
	remaining := cooldownDuration - time.Since(info.Timestamp)
	if remaining <= 0 {
		// Cooldown expired while we were checking — check cache before retrying.
		ClearAuthCooldown(baseUrl, admin)
		if result := cachedTokenResult(baseUrl); result != nil {
			return *result
		}
		return TokenViaListener(baseUrl, admin, cmd, defaultPort, 180*time.Second)
	}

	log.Printf("auth cooldown: waiting for active auth process (PID %d, port %d) to complete (up to %s)",
		info.PID, info.Port, remaining.Truncate(time.Second))

	if WaitForPidExit(info.PID, remaining, 500*time.Millisecond) {
		// Holder finished — use cached credentials if available, otherwise retry.
		if result := cachedTokenResult(baseUrl); result != nil {
			return *result
		}
		return TokenViaListener(baseUrl, admin, cmd, defaultPort, 180*time.Second)
	}

	// Timed out waiting.
	return TokenResult{err: fmt.Errorf(
		"authentication for %s is being handled by another process (PID %d) which has not completed\n"+
			"To force a new attempt: duplo-jit clear-cache  (or unset %s to disable cooldown)",
		GetHostCacheKey(baseUrl), info.PID, authCooldownEnvVar)}
}
