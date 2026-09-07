//go:build linux || darwin || cosmo

package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ps lookup is only USED by an APE on a macOS host, which no CI runner
// is. It is compiled and tested on linux and darwin anyway: both have a ps
// that reports the same two fields, and both have a native lookup to check it
// against, so the implementation an APE depends on is not one nobody runs.

func TestParsePS(t *testing.T) {
	tests := []struct {
		name     string
		out      string
		wantComm string
		wantPPID int
		wantOK   bool
	}{
		{
			name:     "macOS: right-aligned ppid then the accounting name",
			out:      "  1234 claude\n",
			wantComm: "claude",
			wantPPID: 1234,
			wantOK:   true,
		},
		{
			name:     "linux procps prints the same two fields",
			out:      " 2540 bash\n",
			wantComm: "bash",
			wantPPID: 2540,
			wantOK:   true,
		},
		{
			name:     "launchd's parent is pid 0",
			out:      "    0 launchd\n",
			wantComm: "launchd",
			wantPPID: 0,
			wantOK:   true,
		},
		{
			name:     "a process name containing spaces runs to end of line",
			out:      "  501 Google Chrome H\n",
			wantComm: "Google Chrome H",
			wantPPID: 501,
			wantOK:   true,
		},
		{
			name:     "no trailing newline",
			out:      "  1 kernel_task",
			wantComm: "kernel_task",
			wantPPID: 1,
			wantOK:   true,
		},
		{
			name:     "only the first row is read",
			out:      "  1 first\n  2 second\n",
			wantComm: "first",
			wantPPID: 1,
			wantOK:   true,
		},
		{name: "a gone pid prints nothing", out: ""},
		{name: "whitespace only", out: "   \n"},
		{name: "a ppid with no name", out: "  1234\n"},
		{name: "a name with no ppid", out: "claude\n"},
		{name: "a non-numeric first field", out: "PPID UCOMM\n"},
		{name: "a negative ppid is not a pid", out: "  -1 claude\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comm, ppid, ok := parsePS(tt.out)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantComm, comm)
			assert.Equal(t, tt.wantPPID, ppid)
		})
	}
}

// The claim this test defends is the one an APE on a Mac rests on: asking ps
// gives the same answer the platform's own lookup does, so a walk resolves
// identically whichever one ran.
func TestCommPPIDPSAgreesWithTheNativeLookup(t *testing.T) {
	requirePS(t)

	comm, ppid, ok := commPPIDPS(os.Getpid())
	require.True(t, ok, "ps could not resolve this process")

	wantComm, wantPPID, wantOK := CommPPID(os.Getpid())
	require.True(t, wantOK)
	assert.Equal(t, wantComm, comm)
	assert.Equal(t, wantPPID, ppid)
	assert.Equal(t, os.Getppid(), ppid)
}

func TestCommPPIDPSOnAGonePID(t *testing.T) {
	requirePS(t)

	_, _, ok := commPPIDPS(1 << 30)
	assert.False(t, ok)
}

// A pid that cannot exist is answered without running anything.
func TestCommPPIDPSRejectsNonPIDs(t *testing.T) {
	for _, pid := range []int{0, -1} {
		_, _, ok := commPPIDPS(pid)
		assert.False(t, ok, "pid %d", pid)
	}
}

// A ps that never answers must not hang the caller. The script stands in for
// a ps stuck in the kernel: it hands its stdout to a grandchild and sleeps,
// so the kill at the deadline leaves the pipe open. Without WaitDelay the
// lookup blocks until the grandchild exits.
func TestCommPPIDPSGivesUpOnAHungPS(t *testing.T) {
	// Same reason as the test below: this one swaps psBin too.
	t.Serial()
	script := filepath.Join(t.TempDir(), "ps")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nsleep 20 &\nsleep 20\n"), 0o755))
	oldBin, oldTimeout := psBin, psTimeout
	psBin, psTimeout = script, 200*time.Millisecond
	t.Cleanup(func() { psBin, psTimeout = oldBin, oldTimeout })

	start := time.Now()
	_, _, ok := commPPIDPS(os.Getpid())
	assert.False(t, ok)
	assert.Less(t, time.Since(start), 5*time.Second, "the lookup waited for the grandchild instead of giving up")
}

// The three ways a lookup answers nothing want three different repairs, so
// each has to arrive as its own sentence. A caller reading only ok cannot
// tell a refused reader from a process that has already exited.
func TestCommPPIDPSSaysWhyItAnsweredNothing(t *testing.T) {
	// psBin is package state, and a top-level test in this fork starts
	// parallel. Swapping it under a sibling makes that sibling read a fake.
	t.Serial()
	dir := t.TempDir()
	oldBin, oldTimeout := psBin, psTimeout
	t.Cleanup(func() { psBin, psTimeout = oldBin, oldTimeout })

	t.Run("a ps that cannot be started", func(t *testing.T) {
		psBin = filepath.Join(dir, "no-such-ps")
		_, _, ok := commPPIDPS(os.Getpid())
		require.False(t, ok)
		assert.Contains(t, LookupError(), "could not be run")
	})

	t.Run("a ps that refuses", func(t *testing.T) {
		script := filepath.Join(dir, "refusing-ps")
		require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho 'operation not permitted' >&2\nexit 1\n"), 0o755))
		psBin = script
		_, _, ok := commPPIDPS(os.Getpid())
		require.False(t, ok)
		// The reason ps gave is the whole point: a sandbox says so on stderr.
		assert.Contains(t, LookupError(), "operation not permitted")
	})

	t.Run("a ps that ran and knew nothing", func(t *testing.T) {
		script := filepath.Join(dir, "silent-ps")
		require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755))
		psBin = script
		_, _, ok := commPPIDPS(os.Getpid())
		require.False(t, ok)
		assert.Contains(t, LookupError(), "printed no row")
	})

	t.Run("an answer clears the last reason", func(t *testing.T) {
		script := filepath.Join(dir, "answering-ps")
		require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho '  42 launchd'\n"), 0o755))
		psBin = script
		comm, ppid, ok := commPPIDPS(os.Getpid())
		require.True(t, ok)
		assert.Equal(t, "launchd", comm)
		assert.Equal(t, 42, ppid)
		assert.Empty(t, LookupError(), "a stale reason names a problem that is no longer there")
	})
}

func requirePS(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(psPath); err != nil {
		t.Skipf("%s is not installed on this machine: %v", psPath, err)
	}
}
