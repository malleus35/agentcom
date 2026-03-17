//go:build !windows

package cli

import (
	"os"
	"syscall"
)

func supervisorSignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR1, syscall.SIGHUP}
}

func supervisorSignalAction(sig os.Signal) string {
	if sig == syscall.SIGUSR1 {
		return supervisorSignalDumpState
	}
	return supervisorSignalReload
}
