//go:build linux || darwin || cosmo

// The process-tree entry points, for every platform that has a lookup to
// build them on. Which lookup CommPPID is comes from the platform: /proc
// (proc_linux.go), sysctl (proc_darwin.go), or a host dispatch between /proc
// and ps (proc_cosmo.go). proc_other.go carries the stubs for the rest.

package agent

import "os"

// ProcessAncestor returns the agent owning an ancestor of this process.
func ProcessAncestor() (Agent, bool) {
	return AncestorAgent(os.Getppid(), CommPPID)
}

// IsAncestorPID reports whether target is somewhere in this process's
// parent-PID chain.
func IsAncestorPID(target int) bool {
	return AncestorPID(os.Getppid(), target, CommPPID)
}

// IsPipeReader reports whether the process (comm, pid) reading one of our
// pipes is an agent capturing our output. See PipeReader.
func IsPipeReader(comm string, pid int) bool {
	return PipeReader(comm, pid, os.Getppid(), CommPPID)
}
