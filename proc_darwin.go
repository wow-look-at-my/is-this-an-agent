//go:build darwin

// The sysctl-backed lookup for native darwin builds (a GOOS=cosmo fat APE
// still uses proc.go's /proc path on darwin hosts, same as it does on linux --
// see proc.go's own comment). darwin has no /proc, but KERN_PROC over sysctl
// answers the same question: a process's comm and parent pid, via the same
// kinfo_proc struct macOS's own ps/lsof read.

package agent

import (
	"os"

	"golang.org/x/sys/unix"
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

// CommPPID looks up a process's comm and parent pid via sysctl(KERN_PROC,
// KERN_PROC_PID), the same source `ps` and `lsof` read on macOS. ok is false
// if the pid is gone or the sysctl fails -- never a guess.
func CommPPID(pid int) (comm string, ppid int, ok bool) {
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", 0, false
	}
	name := kp.Proc.P_comm[:]
	n := 0
	for n < len(name) && name[n] != 0 {
		n++
	}
	return string(name[:n]), int(kp.Eproc.Ppid), true
}
