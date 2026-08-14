//go:build cosmo

package agent

import (
	"sync"
	"syscall"
)

// A GOOS=cosmo binary is one fat APE that boots natively on Linux and macOS
// hosts alike, reporting runtime.GOOS == "cosmo" on both, so the host has to
// be probed at runtime. The result is memoized: a process cannot change hosts.
var hostOS = sync.OnceValue(func() string { return hostFromEvidence(gatherHostEvidence()) })

// HostOS returns the operating system this APE is running on: "linux",
// "darwin", or "" when the probe could not identify it.
//
// "" is a real answer and callers must handle it. It never means linux; it
// means every signal was refused, which is what a sufficiently restrictive
// sandbox produces. A caller that needs a host-specific resource should
// refuse or degrade on "" rather than pick one.
//
// Never "windows": on a Windows host a fat APE runs its embedded native
// GOOS=windows payload, which compiles host_other.go instead.
func HostOS() string { return hostOS() }

// gatherHostEvidence collects every host signal available to a cosmo binary.
// Each one is independent, and each is recorded rather than acted on, so
// hostFromEvidence weighs them all in one place.
func gatherHostEvidence() hostEvidence {
	return hostEvidence{
		runtimeHost:  runtimeHostOS(),
		unameSysname: unameSysname(),
		procSelf:     statPath("/proc/self"),
		coreServices: statPath("/System/Library/CoreServices"),
	}
}

// runtimeHostOS returns the host the APE loader booted this binary for, as
// the cosmo runtime recorded it at rt0, and "" when the toolchain does not
// expose it.
//
// This is the signal worth having: the runtime dispatches every syscall on
// that value, so it is neither a probe nor an inference, and no sandbox can
// interfere with it. It becomes available when the gosmopolitan toolchain
// exports it (as runtime.CosmoHostOS); the body here is what a toolchain that
// predates the export can say, and the rest of the evidence covers that gap.
func runtimeHostOS() string { return "" }

// unameSysname returns uname(2)'s sysname, or "" when the call failed.
//
// It is a syscall, so a filesystem sandbox cannot touch it, and under cosmo
// its two outcomes are both informative. On a Linux host the raw Linux-numbered
// syscall passes straight through to the kernel and names Linux. On a macOS
// host the runtime's darwin dispatcher has no case for it and answers ENOSYS,
// which is why hostFromEvidence treats silence here as evidence for darwin
// rather than as a dead end.
func unameSysname() string {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return ""
	}
	name := uts.Sysname[:]
	for i, c := range name {
		if c == 0 {
			return string(name[:i])
		}
	}
	return string(name)
}
