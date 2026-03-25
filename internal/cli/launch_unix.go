//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func launchProcess(binary string, args []string, env []string) error {
	if len(env) == 0 {
		env = os.Environ()
	}
	argv := append([]string{binary}, args...)
	return syscall.Exec(binary, argv, env)
}
