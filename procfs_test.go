//go:build linux || cosmo

package agent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommPPIDProcSelfProcess(t *testing.T) {
	comm, ppid, ok := commPPIDProc(os.Getpid())
	require.True(t, ok)
	assert.NotEmpty(t, comm)
	assert.Equal(t, os.Getppid(), ppid)
}

func TestCommPPIDProcNonexistentPID(t *testing.T) {
	_, _, ok := commPPIDProc(1 << 30)
	assert.False(t, ok)
}

// The blind-sandbox case, run against the real lookups rather than fakes: a
// host the probe could not identify still resolves pids, because the fallback
// asks both lookups instead of picking one. On the machine running this test
// /proc answers; on a macOS host the same code reaches ps.
func TestAnUndeterminedHostStillResolvesAPID(t *testing.T) {
	comm, ppid, ok := lookupForHost(hostUnknown, commPPIDProc, commPPIDPS)(os.Getpid())
	require.True(t, ok, "neither lookup could resolve this process")
	assert.NotEmpty(t, comm)
	assert.Equal(t, os.Getppid(), ppid)
}
