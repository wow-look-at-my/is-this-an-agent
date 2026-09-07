//go:build linux || darwin || cosmo

package agent

import (
	"context"
	"errors"
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
// its parent was. Both are variables so a test can point the lookup at a
// script and shorten the wait.
var (
	psBin     = psPath
	psTimeout = 5 * time.Second
)

// commPPIDPS looks up a process's comm and parent pid by running ps. ok is
// false if the pid is gone or ps cannot be run -- never a guess.
//
// The requested fields match what the other two lookups report: ucomm is the
// accounting name, the same MAXCOMLEN-truncated string sysctl returns in
// kinfo_proc.P_comm and Linux reports as /proc comm, so process-name prefixes
// match identically whichever lookup answered.
func commPPIDPS(pid int) (comm string, ppid int, ok bool) {
	if pid <= 0 {
		noteLookupErr("ps was asked for pid " + strconv.Itoa(pid) + ", which is not a pid")
		return "", 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, psBin, "-o", "ppid=,ucomm=", "-p", strconv.Itoa(pid))
	// The kill at the deadline does not end Wait while the stdout pipe stays
	// open, and a ps stuck in the kernel keeps it open. WaitDelay closes it.
	cmd.WaitDelay = time.Second
	out, err := cmd.Output()
	if err != nil {
		noteLookupErr(psFailure(pid, ctx.Err(), err))
		return "", 0, false
	}
	comm, ppid, ok = parsePS(string(out))
	if !ok {
		// ps ran and said nothing about the pid. That is what it does for a
		// process that has already exited, and it is not a broken reader.
		noteLookupErr(psBin + " printed no row for pid " + strconv.Itoa(pid))
		return "", 0, false
	}
	clearLookupErr()
	return comm, ppid, true
}

// psFailure names why running ps did not produce output. The three causes
// want different repairs: a deadline says the host is wedged, an exit
// status says ps refused, and anything else says it could not be started
// at all -- which on a sandboxed host is the interesting one.
func psFailure(pid int, ctxErr, err error) string {
	at := " looking up pid " + strconv.Itoa(pid)
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return psBin + at + " did not answer within " + psTimeout.String()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		reason := psBin + at + " exited " + exit.ProcessState.String()
		if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
			reason += ": " + stderr
		}
		return reason
	}
	return psBin + at + " could not be run: " + err.Error()
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
