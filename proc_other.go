//go:build !linux && !cosmo

// Stubs for platforms with no /proc (native darwin and windows builds): the
// LOOKUP is missing here, not the matching rules -- AncestorAgent and friends
// in walk.go take a chain from the caller and work everywhere. Detect falls
// back to the agents' environment markers when ProcessAncestor reports
// nothing, which is what makes detection still work here.
//
// These must never shadow the real implementations: the released "linux"
// binaries of this org's tools are GOOS=cosmo APE copies, and cosmo is
// excluded from this file for exactly that reason (see proc.go).

package agent

// ProcessAncestor cannot walk a parent chain without /proc.
func ProcessAncestor() (Agent, bool) { return Agent{}, false }

// IsAncestorPID cannot answer without /proc, and answers false rather than
// guessing: callers use it to grant an allowance ("the agent is reading this
// pipe"), so an unknowable answer must not grant it.
func IsAncestorPID(int) bool { return false }

// IsPipeReader likewise cannot identify the far end of a pipe here.
func IsPipeReader(string, int) bool { return false }

// CommPPID has no /proc to read.
func CommPPID(int) (comm string, ppid int, ok bool) { return "", 0, false }
