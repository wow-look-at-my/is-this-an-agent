//go:build darwin

package agent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommPPID_SelfProcess(t *testing.T) {
	comm, ppid, ok := CommPPID(os.Getpid())
	require.True(t, ok)
	assert.NotEmpty(t, comm)
	assert.Equal(t, os.Getppid(), ppid)
}

func TestCommPPID_NonexistentPID(t *testing.T) {
	_, _, ok := CommPPID(1 << 30)
	assert.False(t, ok)
}
