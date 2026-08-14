package agent

// The host OS a binary is running ON, as opposed to the OS it was compiled
// FOR. Those are the same thing everywhere except a GOOS=cosmo fat APE, which
// is one binary that boots natively on both Linux and macOS while
// runtime.GOOS stays "cosmo" on either -- so a cosmo build has to pick its
// process lookup at runtime instead of at compile time.
//
// Gathering the evidence is platform-specific (host_cosmo.go). Weighing it is
// not: hostFromEvidence takes the evidence as data, so every host it can face
// -- including the sandboxed ones, where the probe is at its weakest -- is a
// test case that runs on every platform.

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// The host OS names HostOS reports. They are runtime.GOOS spellings, so a
// caller can compare against either interchangeably.
const (
	hostLinux  = "linux"
	hostDarwin = "darwin"

	// hostUnknown is the answer when no evidence identifies the host. It is
	// never a stand-in for a host that was not probed successfully: nothing
	// in this package treats it as linux.
	hostUnknown = ""
)

// The signals HostOS can decide from, reported by HostSource. A detector that
// says HOW it decided is one that can be read off a log line instead of
// reasoned about: "denied probe" and "read the machine" look identical in an
// answer, and different in a source.
const (
	sourceGOOS         = "goos"         // compiled for the host it runs on
	sourceRuntime      = "runtime"      // the host the APE loader booted
	sourceUname        = "uname"        // uname(2) named it
	sourceProcfs       = "procfs"       // /proc/self is there
	sourceCoreServices = "coreservices" // /System/Library/CoreServices is there
	sourceNoProcfs     = "no-procfs"    // /proc/self is definitively absent
	sourceNone         = ""             // nothing identified the host
)

// pathState is what a stat of a probe path established. The three-way split
// is the whole point: a sandbox answers a denied path with EPERM, and reading
// that as "absent" is how a probe concludes "linux" on a Mac.
type pathState int

const (
	// pathDenied means the stat failed for a reason other than absence. It
	// is evidence of nothing at all.
	pathDenied pathState = iota
	pathPresent
	pathAbsent
)

// statPath reports whether path exists, distinguishing a definite absence
// from a stat that was refused.
func statPath(path string) pathState {
	if _, err := os.Stat(path); err == nil {
		return pathPresent
	} else if errors.Is(err, fs.ErrNotExist) {
		return pathAbsent
	}
	return pathDenied
}

// hostEvidence is everything the probe collected about the host.
type hostEvidence struct {
	// runtimeHost is the host the APE loader booted this binary for, as the
	// cosmo runtime recorded it, or "" when the toolchain does not expose
	// it. It is the only signal that is not an inference: the runtime
	// dispatches every syscall on it, so it cannot be denied, refused or
	// raced. Nothing below it is consulted when it is set.
	runtimeHost string

	// unameSysname is uname(2)'s sysname, or "" when the call failed. A
	// cosmo binary's uname is a raw Linux syscall that passes straight
	// through on a Linux host; on a macOS host the runtime's darwin
	// dispatcher has no case for it and answers ENOSYS.
	unameSysname string

	// procSelf is /proc/self, present on every Linux and on no macOS.
	procSelf pathState

	// coreServices is /System/Library/CoreServices, present on every macOS
	// and on no Linux.
	coreServices pathState
}

// hostFromEvidence names the host OS and the signal that named it, or
// hostUnknown and sourceNone when the evidence identifies nothing.
//
// The rungs are ordered by how much a sandbox can interfere with them. What
// the runtime was booted with is fact and outranks everything. uname is a
// syscall, so no filesystem policy can change its answer. The path checks
// below it only ever fire on a successful stat, because a refused one is not
// evidence -- and the last rung reads a definite procfs ENOENT as ruling Linux
// out, so a macOS host whose /System/Library cannot be read is still darwin.
//
// No rung ends in a default. Evidence that identifies nothing yields
// hostUnknown, because a host that answers "linux" because nothing could be
// read is the exact failure this probe exists to avoid.
func hostFromEvidence(e hostEvidence) (host, source string) {
	switch e.runtimeHost {
	case hostLinux, hostDarwin:
		return e.runtimeHost, sourceRuntime
	}

	switch strings.ToLower(e.unameSysname) {
	case "linux":
		return hostLinux, sourceUname
	case "darwin", "xnu":
		return hostDarwin, sourceUname
	case "":
	default:
		// uname named an OS this package does not dispatch for. Its
		// silence is the darwin signal the last rung relies on, so a
		// name that is not darwin's must not fall through to it.
		return hostUnknown, sourceNone
	}

	if e.procSelf == pathPresent {
		return hostLinux, sourceProcfs
	}
	if e.coreServices == pathPresent {
		return hostDarwin, sourceCoreServices
	}
	if e.procSelf == pathAbsent {
		return hostDarwin, sourceNoProcfs
	}
	return hostUnknown, sourceNone
}

// lookupForHost picks the process lookup for a host OS: /proc on Linux,
// ps(1) on macOS.
//
// An undetermined host does not get a guess. Both lookups are tried, because
// neither can answer wrongly on the other's host -- /proc/<pid>/stat does not
// exist on macOS, and ps reports the same two fields on both -- so asking
// both is an answer where picking one is a coin flip.
func lookupForHost(host string, procfs, ps Lookup) Lookup {
	switch host {
	case hostLinux:
		return procfs
	case hostDarwin:
		return ps
	}
	return func(pid int) (string, int, bool) {
		if comm, ppid, ok := procfs(pid); ok {
			return comm, ppid, true
		}
		return ps(pid)
	}
}
