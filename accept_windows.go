package socks5lb

import (
	"errors"

	"golang.org/x/sys/windows"
)

func isResourceAcceptError(err error) bool {
	return errors.Is(err, windows.WSAEMFILE) ||
		errors.Is(err, windows.WSAENOBUFS) ||
		errors.Is(err, windows.ERROR_TOO_MANY_OPEN_FILES) ||
		errors.Is(err, windows.ERROR_NO_SYSTEM_RESOURCES) ||
		errors.Is(err, windows.ERROR_NOT_ENOUGH_MEMORY) ||
		errors.Is(err, windows.ERROR_OUTOFMEMORY) ||
		errors.Is(err, windows.WSAECONNABORTED) ||
		errors.Is(err, windows.WSAECONNRESET)
}
