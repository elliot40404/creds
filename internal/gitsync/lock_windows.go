package gitsync

import (
	"errors"
	"math"

	"golang.org/x/sys/windows"
)

const stillActive = 259

func processAlive(pid int) bool {
	if pid <= 0 || pid > math.MaxUint32 {
		return true
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false
	}
	if err != nil {
		return true
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}
