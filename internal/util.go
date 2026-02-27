package internal

import (
	"errors"
	"log"
	"os"
	"syscall"
	"time"
)

func DieIf(err error, msg string) {
	if err != nil {
		Fatal(msg, err)
	}
}

func Fatal(msg string, err error) {
	if err != nil {
		log.Fatalf("%s: %s: %s", os.Args[0], msg, err)
	}
	log.Fatalf("%s: %s", os.Args[0], msg)
}

// IsPidAlive checks whether a process with the given PID is still running.
func IsPidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, os.ErrPermission)
}

// WaitForPidExit polls until the given PID exits or the timeout is reached.
// Returns true if the PID exited, false if the timeout was reached.
func WaitForPidExit(pid int, timeout time.Duration, pollInterval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsPidAlive(pid) {
			return true
		}
		time.Sleep(pollInterval)
	}
	return !IsPidAlive(pid)
}
