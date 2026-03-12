//go:build windows

package cli

import "syscall"

func processAliveCheck(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)

	return true
}
