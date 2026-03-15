//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func configureDetachedProcess(cmd *exec.Cmd) error {
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("cli.configureDetachedProcess: open devnull: %w", err)
	}
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return nil
}
