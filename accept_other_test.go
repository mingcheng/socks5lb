//go:build !windows

package socks5lb

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceAcceptErrors(t *testing.T) {
	for _, err := range []error{syscall.EMFILE, syscall.ENFILE, syscall.ENOBUFS, syscall.ENOMEM, syscall.ECONNABORTED, syscall.ECONNRESET} {
		require.True(t, isResourceAcceptError(fmt.Errorf("accept: %w", err)))
	}
	require.False(t, isResourceAcceptError(syscall.EINVAL))
	require.False(t, isResourceAcceptError(errors.New("too many open files")))
}
