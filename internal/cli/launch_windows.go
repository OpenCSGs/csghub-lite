//go:build windows

package cli

import (
	"os"
	"os/exec"
)

func launchProcess(binary string, args []string, env []string) error {
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(env) > 0 {
		cmd.Env = env
	}
	return cmd.Run()
}
