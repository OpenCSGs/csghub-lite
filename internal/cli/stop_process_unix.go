//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func stopProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}

func processExists(proc *os.Process) bool {
	return proc.Signal(syscall.Signal(0)) == nil
}
