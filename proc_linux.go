//go:build linux

package agent

// A native Linux build reads /proc directly: it is compiled for the host it
// runs on, so there is no host to probe. The `linux` constraint here selects
// an implementation and nothing else -- the /proc reader itself lives in
// procfs.go under `linux || cosmo`, where an APE can reach it too.

// CommPPID reads /proc/<pid>/stat and returns the process's comm and its
// parent PID. ok is false if the entry cannot be read or parsed.
func CommPPID(pid int) (comm string, ppid int, ok bool) { return commPPIDProc(pid) }
