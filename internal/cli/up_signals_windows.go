//go:build windows

package cli

import "os"

func supervisorSignals() []os.Signal {
	return []os.Signal{}
}

func supervisorSignalAction(os.Signal) string {
	return supervisorSignalReload
}
