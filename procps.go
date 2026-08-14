//go:build linux || darwin || cosmo

package agent

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The ps(1)-backed lookup: the process-tree lookup for a GOOS=cosmo APE
// running on a macOS host.
//
// macOS has no /proc, and the sysctl(KERN_PROC) call proc_darwin.go uses is
// out of reach from a cosmo binary in both directions: golang.org/x/sys/unix
// has no cosmo port, and the fork runtime's darwin dispatcher emulates no
// sysctl, so a raw one answers ENOSYS. ps reads the same kinfo_proc the
// sysctl does, through the OS's own tool, and it is reachable because os/exec
// works on a cosmo APE on macOS.
//
// It is compiled on darwin and linux as well as cosmo, unused by either
// except in tests -- which is how the one lookup no CI runner can execute in
// its real configuration is checked against the two that can.

// psPath is where macOS and every mainstream Linux keep ps. It is absolute so
// the lookup does not depend on PATH.
const psPath = "/bin/ps"

// psTimeout bounds the lookup. ps answers instantly or not at all; the bound
// exists so a wedged host cannot hang a caller that only wanted to know who
// its parent was.
const psTimeout = 5 * time.Second

// commPPIDPS looks up a process's comm and parent pid by running ps. ok is
// false if the pid is gone or ps cannot be run -- never a guess.
//
// The requested fields match what the other two lookups report: ucomm is the
// accounting name, the same MAXCOMLEN-truncated string sysctl returns in
// kinfo_proc.P_comm and Linux reports as /proc comm, so process-name prefixes
// match identically whichever lookup answered.
func commPPIDPS(pid int) (comm string, ppid int, ok bool) {
	if pid <= 0 {
		return "", 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, psPath, "-o", "ppid=,ucomm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", 0, false
	}
	return parsePS(string(out))
}

// parsePS reads one `ps -o ppid=,ucomm=` row: a right-aligned parent pid, a
// space, then the accounting name, which runs to the end of the line because
// a process name may itself contain spaces.
func parsePS(out string) (comm string, ppid int, ok bool) {
	line, _, _ := strings.Cut(out, "\n")
	line = strings.TrimSpace(line)
	sep := strings.IndexAny(line, " \t")
	if sep < 0 {
		return "", 0, false
	}
	ppid, err := strconv.Atoi(line[:sep])
	if err != nil || ppid < 0 {
		return "", 0, false
	}
	comm = strings.TrimSpace(line[sep:])
	if comm == "" {
		return "", 0, false
	}
	return comm, ppid, true
}
