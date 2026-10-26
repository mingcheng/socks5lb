//go:build !race

package socks5lb

import (
	"fmt"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Race instrumentation deliberately discards sync.Pool entries, so allocation
// assertions run separately from the functional and concurrency race tests.
func TestPoolNextSteadyStateAllocations(t *testing.T) {
	level := log.GetLevel()
	log.SetLevel(log.ErrorLevel)
	t.Cleanup(func() { log.SetLevel(level) })
	pool := NewPool()
	for i := range 256 {
		require.NoError(t, pool.Add(NewBackend(fmt.Sprintf("backend-%03d", i), BackendCheckConfig{
			InitialAlive: i%2 == 0,
		})))
	}
	allocations := testing.AllocsPerRun(100, func() {
		if pool.Next() == nil {
			t.Fatal("no healthy backend")
		}
	})
	require.Zero(t, allocations, "steady-state selection must not allocate")
}
