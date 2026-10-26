package socks5lb

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestResourceAcceptErrors(t *testing.T) {
	for _, err := range []error{
		windows.WSAEMFILE, windows.WSAENOBUFS, windows.ERROR_NOT_ENOUGH_MEMORY,
		windows.ERROR_OUTOFMEMORY, windows.WSAECONNABORTED, windows.WSAECONNRESET,
		windows.ERROR_TOO_MANY_OPEN_FILES, windows.ERROR_NO_SYSTEM_RESOURCES,
	} {
		require.True(t, isResourceAcceptError(fmt.Errorf("accept: %w", err)))
	}
	require.False(t, isResourceAcceptError(windows.WSAEINVAL))
	require.False(t, isResourceAcceptError(errors.New("too many open files")))
}
