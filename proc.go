//go:build linux || cosmo

// The /proc-backed lookup, and the process-tree entry points built on it.
//
// The build constraint is `linux || cosmo`, not `linux`, and that matters: a
// GOOS=cosmo binary (a gosmopolitan fat APE) runs on Linux hosts and has a real
// /proc there, but cosmo matches the `unix` build tag rather than `linux` -- so
// a `_linux.go` filename would silently compile every ancestry check out of an
// APE while the GOOS=linux tests stayed green. proc_other.go carries the stubs
// for everything else.

package agent

import (
	"os"
	"strconv"
	"strings"
)

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

// CommPPID reads /proc/<pid>/stat and returns the process's comm and its
// parent PID. ok is false if the entry cannot be read or parsed.
func CommPPID(pid int) (comm string, ppid int, ok bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", 0, false
	}
	s := string(data)
	// Field layout: "<pid> (<comm>) <state> <ppid> ...". comm may itself
	// contain spaces and parentheses, so anchor on the LAST ')' rather than
	// splitting the whole line into fields.
	open := strings.IndexByte(s, '(')
	closeParen := strings.LastIndexByte(s, ')')
	if open < 0 || closeParen < open {
		return "", 0, false
	}
	comm = s[open+1 : closeParen]
	fields := strings.Fields(s[closeParen+1:]) // [state, ppid, ...]
	if len(fields) < 2 {
		return "", 0, false
	}
	ppid, err = strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, false
	}
	return comm, ppid, true
}
