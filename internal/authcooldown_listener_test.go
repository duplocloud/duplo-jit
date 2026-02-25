package internal

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestCheckCooldownBeforeListen_NoCooldown(t *testing.T) {
	host := setupTestHost(t)

	port, browser, timeout, result := checkCooldownBeforeListen(host, false, "test", 0, 60*time.Minute)
	if result != nil {
		t.Fatal("expected no early result")
	}
	if port != 0 {
		t.Errorf("expected port 0, got %d", port)
	}
	if !browser {
		t.Error("expected openBrowser=true")
	}
	if timeout != 60*time.Minute {
		t.Errorf("expected timeout 60m, got %v", timeout)
	}
}

func TestCheckCooldownBeforeListen_ExpiredCooldown(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	// Create expired cooldown file.
	writeFakeCooldown(t, host, false, 99999, 54321, time.Now().Add(-cooldownDuration-time.Second))

	port, browser, timeout, result := checkCooldownBeforeListen(host, false, "test", 0, cooldownDuration)
	if result != nil {
		t.Fatal("expected no early result for expired cooldown")
	}
	if port != 0 {
		t.Errorf("expected default port 0, got %d", port)
	}
	if !browser {
		t.Error("expected openBrowser=true")
	}
	if timeout != cooldownDuration {
		t.Errorf("expected timeout %v, got %v", cooldownDuration, timeout)
	}

	// Verify cooldown was cleared.
	if ReadCooldownInfo(host, false) != nil {
		t.Error("expected cooldown to be cleared after expiry")
	}
}

func TestCheckCooldownBeforeListen_DeadPid(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	// Create cooldown with a PID that is almost certainly not running.
	writeFakeCooldown(t, host, false, 2147483647, 54321, time.Now().Add(-10*time.Minute))

	port, browser, timeout, result := checkCooldownBeforeListen(host, false, "test", 0, cooldownDuration)
	if result != nil {
		t.Fatal("expected no early result for dead PID relay")
	}
	if port != 54321 {
		t.Errorf("expected relay port 54321, got %d", port)
	}
	if browser {
		t.Error("expected openBrowser=false for relay")
	}
	if timeout >= cooldownDuration {
		t.Errorf("expected timeout < cooldownDuration, got %v", timeout)
	}
}

func TestCheckCooldownBeforeListen_AdminIndependent(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	// Create non-admin cooldown with dead PID.
	writeFakeCooldown(t, host, false, 2147483647, 54321, time.Now().Add(-10*time.Minute))

	// Admin check should see no cooldown.
	port, browser, _, result := checkCooldownBeforeListen(host, true, "test", 0, cooldownDuration)
	if result != nil {
		t.Fatal("expected no early result for admin (independent cooldown)")
	}
	if port != 0 {
		t.Errorf("expected default port for admin, got %d", port)
	}
	if !browser {
		t.Error("expected openBrowser=true for admin (no cooldown)")
	}
}

func TestAcquireOrUpdateCooldown_FreshStart(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()
	localPort := listener.Addr().(*net.TCPAddr).Port

	result := acquireOrUpdateCooldown(host, false, "test", 0, localPort, true, cooldownDuration, listener)
	if result != nil {
		t.Fatalf("expected nil result for fresh start, got err: %v", result.err)
	}

	// Verify cooldown was created.
	info := ReadCooldownInfo(host, false)
	if info == nil {
		t.Fatal("expected cooldown info to exist after fresh start")
	}
	if info.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
	if info.Port != localPort {
		t.Errorf("expected port %d, got %d", localPort, info.Port)
	}
}

func TestAcquireOrUpdateCooldown_RelayUpdate(t *testing.T) {
	host := setupTestHost(t)
	cooldownDuration := 60 * time.Minute

	// Create initial cooldown (simulating a previous process).
	mustSetCooldown(t, host, 8080, false, cooldownDuration)
	originalInfo := ReadCooldownInfo(host, false)
	if originalInfo == nil {
		t.Fatal("expected initial cooldown info")
	}

	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("failed to create listener: %v", listenErr)
	}
	defer func() { _ = listener.Close() }()
	localPort := listener.Addr().(*net.TCPAddr).Port

	// Relay path: openBrowser=false.
	result := acquireOrUpdateCooldown(host, false, "test", 0, localPort, false, cooldownDuration, listener)
	if result != nil {
		t.Fatalf("expected nil result for relay, got err: %v", result.err)
	}

	// Verify cooldown was updated with new PID and port but preserved timestamp.
	info := ReadCooldownInfo(host, false)
	if info == nil {
		t.Fatal("expected cooldown info after relay update")
	}
	if info.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), info.PID)
	}
	if info.Port != localPort {
		t.Errorf("expected port %d, got %d", localPort, info.Port)
	}
	if !info.Timestamp.Equal(originalInfo.Timestamp) {
		t.Errorf("expected preserved timestamp %v, got %v", originalInfo.Timestamp, info.Timestamp)
	}
}

func TestWaitForPidExit_DeadPid(t *testing.T) {
	// PID 2147483647 is almost certainly not running.
	exited := WaitForPidExit(2147483647, 1*time.Second, 10*time.Millisecond)
	if !exited {
		t.Error("expected dead PID to report exited immediately")
	}
}

func TestWaitForPidExit_LivePid(t *testing.T) {
	// Our own PID is alive; WaitForPidExit should time out.
	start := time.Now()
	exited := WaitForPidExit(os.Getpid(), 50*time.Millisecond, 10*time.Millisecond)
	elapsed := time.Since(start)
	if exited {
		t.Error("expected live PID to not report exited")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected to wait at least 50ms, waited %v", elapsed)
	}
}

func TestIsPidAlive_CurrentProcess(t *testing.T) {
	if !IsPidAlive(os.Getpid()) {
		t.Error("expected current process to be alive")
	}
}

func TestIsPidAlive_DeadProcess(t *testing.T) {
	if IsPidAlive(2147483647) {
		t.Error("expected PID 2147483647 to not be alive")
	}
}

func TestIsPidAlive_InvalidPid(t *testing.T) {
	if IsPidAlive(0) {
		t.Error("expected PID 0 to not be alive")
	}
	if IsPidAlive(-1) {
		t.Error("expected PID -1 to not be alive")
	}
}
