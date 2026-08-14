package agent

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The host probe is the one piece of this package whose real inputs no test
// runner can arrange: the hosts that matter are a cosmo APE on a Mac, and a
// cosmo APE on a Mac inside a sandbox that refuses the paths a probe would
// read. So the decision takes its evidence as data and every one of those
// hosts is a case here, on whatever platform the tests run.

func TestHostFromEvidence(t *testing.T) {
	tests := []struct {
		name       string
		e          hostEvidence
		want       string
		wantSource string
	}{
		{
			name: "linux host: uname names it",
			e: hostEvidence{
				unameSysname: "Linux",
				procSelf:     pathPresent,
				coreServices: pathAbsent,
			},
			want:       hostLinux,
			wantSource: sourceUname,
		},
		{
			// The measured shape on a macos-latest runner, inside the
			// seatbelt sandbox go-toolchain's guard suite runs in: uname
			// ENOSYSes under the darwin dispatcher and CoreServices reads
			// fine, so this rung is the one that answers there.
			name: "macOS host: uname is ENOSYS, CoreServices is readable",
			e: hostEvidence{
				unameSysname: "",
				procSelf:     pathAbsent,
				coreServices: pathPresent,
			},
			want:       hostDarwin,
			wantSource: sourceCoreServices,
		},
		{
			name: "macOS host whose CoreServices is denied: procfs is still definitely absent",
			e: hostEvidence{
				unameSysname: "",
				procSelf:     pathAbsent,
				coreServices: pathDenied,
			},
			want:       hostDarwin,
			wantSource: sourceNoProcfs,
		},
		{
			name: "linux host whose uname is blocked: procfs is still there",
			e: hostEvidence{
				unameSysname: "",
				procSelf:     pathPresent,
				coreServices: pathDenied,
			},
			want:       hostLinux,
			wantSource: sourceProcfs,
		},
		{
			name: "sandbox denies every path and uname: no host is claimed",
			e: hostEvidence{
				unameSysname: "",
				procSelf:     pathDenied,
				coreServices: pathDenied,
			},
			want:       hostUnknown,
			wantSource: sourceNone,
		},
		{
			name: "runtime host outranks evidence that contradicts it",
			e: hostEvidence{
				runtimeHost:  hostDarwin,
				unameSysname: "Linux",
				procSelf:     pathPresent,
				coreServices: pathAbsent,
			},
			want:       hostDarwin,
			wantSource: sourceRuntime,
		},
		{
			name: "runtime host is used when nothing else can be read",
			e: hostEvidence{
				runtimeHost:  hostLinux,
				procSelf:     pathDenied,
				coreServices: pathDenied,
			},
			want:       hostLinux,
			wantSource: sourceRuntime,
		},
		{
			name: "a runtime host this package does not dispatch for is not trusted onward",
			e: hostEvidence{
				runtimeHost:  "windows",
				procSelf:     pathDenied,
				coreServices: pathDenied,
			},
			want:       hostUnknown,
			wantSource: sourceNone,
		},
		{
			name: "uname naming a third OS is not read as darwin, whatever procfs says",
			e: hostEvidence{
				unameSysname: "FreeBSD",
				procSelf:     pathAbsent,
				coreServices: pathAbsent,
			},
			want:       hostUnknown,
			wantSource: sourceNone,
		},
		{
			name:       "uname sysname is matched case-insensitively",
			e:          hostEvidence{unameSysname: "DARWIN"},
			want:       hostDarwin,
			wantSource: sourceUname,
		},
		{
			name:       "an emulated uname naming XNU is darwin",
			e:          hostEvidence{unameSysname: "xnu"},
			want:       hostDarwin,
			wantSource: sourceUname,
		},
		{
			name:       "no evidence at all claims nothing",
			e:          hostEvidence{},
			want:       hostUnknown,
			wantSource: sourceNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, source := hostFromEvidence(tt.e)
			assert.Equal(t, tt.want, host)
			assert.Equal(t, tt.wantSource, source, "the signal that decided it")
		})
	}
}

// A denied stat must never be read as an absent path: that reading is what
// makes a probe answer "linux" on a Mac whose /System/Library it cannot see.
func TestHostFromEvidenceNeverDefaultsToLinux(t *testing.T) {
	for _, e := range []hostEvidence{
		{procSelf: pathDenied, coreServices: pathDenied},
		{unameSysname: "", procSelf: pathDenied, coreServices: pathAbsent},
	} {
		host, source := hostFromEvidence(e)
		assert.NotEqual(t, hostLinux, host, "evidence: %+v", e)
		assert.Equal(t, sourceNone, source, "a host nothing identified must not claim a source")
	}
}

// Every host this package claims has to name the signal that claimed it, and
// every host it does not claim has to name none. A source is only worth
// logging if those two can never be confused.
func TestHostAndSourceAgree(t *testing.T) {
	for _, e := range []hostEvidence{
		{runtimeHost: hostLinux},
		{unameSysname: "Linux"},
		{procSelf: pathPresent},
		{coreServices: pathPresent},
		{procSelf: pathAbsent},
		{procSelf: pathDenied, coreServices: pathDenied},
		{},
	} {
		host, source := hostFromEvidence(e)
		assert.Equal(t, host == hostUnknown, source == sourceNone, "evidence: %+v", e)
	}
}

func TestLookupForHost(t *testing.T) {
	procfs := func(pid int) (string, int, bool) {
		if pid == 1 {
			return "from-procfs", 0, true
		}
		return "", 0, false
	}
	ps := func(pid int) (string, int, bool) {
		if pid == 1 {
			return "from-ps", 0, true
		}
		return "", 0, false
	}

	t.Run("linux reads procfs", func(t *testing.T) {
		comm, _, ok := lookupForHost(hostLinux, procfs, ps)(1)
		require.True(t, ok)
		assert.Equal(t, "from-procfs", comm)
	})

	t.Run("darwin runs ps", func(t *testing.T) {
		comm, _, ok := lookupForHost(hostDarwin, procfs, ps)(1)
		require.True(t, ok)
		assert.Equal(t, "from-ps", comm)
	})

	t.Run("linux does not fall back to ps when procfs cannot answer", func(t *testing.T) {
		_, _, ok := lookupForHost(hostLinux, procfs, ps)(2)
		assert.False(t, ok)
	})

	t.Run("an unknown host asks both rather than guessing one", func(t *testing.T) {
		lookup := lookupForHost(hostUnknown, procfs, ps)
		comm, _, ok := lookup(1)
		require.True(t, ok)
		assert.Equal(t, "from-procfs", comm)

		// Whichever lookup the host actually has is the one that answers:
		// on a Mac the procfs read fails and ps is what resolves the pid.
		deadProcfs := func(int) (string, int, bool) { return "", 0, false }
		comm, _, ok = lookupForHost(hostUnknown, deadProcfs, ps)(1)
		require.True(t, ok)
		assert.Equal(t, "from-ps", comm)
	})

	t.Run("an unknown host with no lookup answering reports failure", func(t *testing.T) {
		_, _, ok := lookupForHost(hostUnknown, procfs, ps)(2)
		assert.False(t, ok)
	})
}

func TestStatPath(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, pathPresent, statPath(dir))
	assert.Equal(t, pathAbsent, statPath(dir+"/no-such-entry"))
}

// HostOS is the host, not the compile target, and those differ only for a
// cosmo APE. Everywhere else the two must stay in step, so a caller can
// compare HostOS() against runtime.GOOS spellings.
func TestHostOSOnANativeBuild(t *testing.T) {
	if runtime.GOOS == "cosmo" {
		t.Skip("a cosmo APE probes its host; see TestHostOSOnAnAPE")
	}
	assert.Equal(t, runtime.GOOS, HostOS())
	assert.Equal(t, sourceGOOS, HostSource())
}

// On an APE the host is whatever this test binary is running on. The suite
// runs on Linux CI, so the probe has to say so -- and a probe that answers ""
// here would be reporting that it could not read a machine it plainly can.
func TestHostOSOnAnAPE(t *testing.T) {
	if runtime.GOOS != "cosmo" {
		t.Skip("not an APE")
	}
	host, source := HostOS(), HostSource()
	t.Logf("host: %s (via %s)", host, source)
	require.NotEqual(t, hostUnknown, host, "the probe identified no host on a machine this test is running on")
	assert.Contains(t, []string{hostLinux, hostDarwin}, host)
	assert.NotEqual(t, sourceNone, source)

	// Cross-check against the filesystem the test itself can see.
	if _, err := os.Stat("/proc/self"); err == nil {
		assert.Equal(t, hostLinux, host)
	}
}
