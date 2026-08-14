//go:build cosmo

package agent

// A GOOS=cosmo fat APE is one binary that boots natively on both Linux and
// macOS, so it cannot pick its process lookup at compile time the way every
// other build does. /proc exists on one host and not the other, and reading
// runtime.GOOS answers "cosmo" on both, which is no answer at all: an APE on
// a Mac that asks /proc resolves nothing, and a caller asking "is the agent
// reading my output?" gets neither yes nor no.

// CommPPID looks up a process's comm and parent PID on whichever host this
// APE is running on: /proc on Linux, ps(1) on macOS. ok is false if the pid
// is gone or the host's lookup could not answer -- never a guess.
func CommPPID(pid int) (comm string, ppid int, ok bool) {
	return lookupForHost(HostOS(), commPPIDProc, commPPIDPS)(pid)
}
