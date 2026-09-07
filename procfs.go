//go:build linux || cosmo

package agent

import (
	"os"
	"strconv"
	"strings"
)

// The /proc-backed lookup. The constraint is `linux || cosmo`, not `linux`: a
// GOOS=cosmo fat APE has a real /proc when it runs on a Linux host, and cosmo
// matches the `unix` build tag rather than `linux`, so a `_linux.go` filename
// would compile the lookup out of every APE while the GOOS=linux tests stayed
// green. A cosmo build reaches this through the host dispatch in proc_cosmo.go,
// which is what keeps it off a macOS host, where there is no /proc to read.

// commPPIDProc reads /proc/<pid>/stat and returns the process's comm and its
// parent PID. ok is false if the entry cannot be read or parsed.
func commPPIDProc(pid int) (comm string, ppid int, ok bool) {
	stat := "/proc/" + strconv.Itoa(pid) + "/stat"
	data, err := os.ReadFile(stat)
	if err != nil {
		// The common case is a process that exited. A permission error, or
		// no /proc at all, is the interesting one, and err says which.
		noteLookupErr("could not read " + stat + ": " + err.Error())
		return "", 0, false
	}
	s := string(data)
	// Field layout: "<pid> (<comm>) <state> <ppid> ...". comm may itself
	// contain spaces and parentheses, so anchor on the LAST ')' rather than
	// splitting the whole line into fields.
	open := strings.IndexByte(s, '(')
	closeParen := strings.LastIndexByte(s, ')')
	if open < 0 || closeParen < open {
		noteLookupErr(stat + " has no parenthesized comm field")
		return "", 0, false
	}
	comm = s[open+1 : closeParen]
	fields := strings.Fields(s[closeParen+1:]) // [state, ppid, ...]
	if len(fields) < 2 {
		noteLookupErr(stat + " ends before the ppid field")
		return "", 0, false
	}
	ppid, err = strconv.Atoi(fields[1])
	if err != nil {
		noteLookupErr(stat + " has a ppid field that is not a number: " + err.Error())
		return "", 0, false
	}
	clearLookupErr()
	return comm, ppid, true
}
