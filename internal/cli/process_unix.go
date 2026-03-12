//go:build !windows

package cli

import (
	"errors"
	"syscall"
)

func processAliveCheck(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}

	return errors.Is(err, syscall.EPERM)
}
