//go:build darwin

// The sysctl-backed lookup for native darwin builds. darwin has no /proc, but
// KERN_PROC over sysctl answers the same question: a process's comm and
// parent pid, via the same kinfo_proc struct macOS's own ps/lsof read.
//
// This is the lookup for a binary compiled FOR darwin. A GOOS=cosmo fat APE
// running ON a macOS host cannot use it: golang.org/x/sys/unix has no cosmo
// port, and the fork runtime's darwin dispatcher emulates no sysctl, so a raw
// one answers ENOSYS. That build reaches the same kinfo_proc fields through
// ps instead (procps.go), picked by the host dispatch in proc_cosmo.go.

package agent

import "golang.org/x/sys/unix"

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
