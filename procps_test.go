//go:build linux || darwin || cosmo

package agent

import (
	"os"
	"testing"

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

func requirePS(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(psPath); err != nil {
		t.Skipf("%s is not installed on this machine: %v", psPath, err)
	}
}
