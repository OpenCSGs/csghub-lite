//go:build windows

package cli

import "os"

func stopProcess(proc *os.Process) error {
	return proc.Kill()
}

func processExists(proc *os.Process) bool {
	_ = proc
	return true
}
