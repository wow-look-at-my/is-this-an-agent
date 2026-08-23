//go:build linux && !cosmo

package agent

// A native Linux build reads /proc directly: it is compiled for the host it
// runs on, so there is no host to probe. The `linux` constraint here selects
// an implementation and nothing else -- the /proc reader itself lives in
// procfs.go under `linux || cosmo`, where an APE can reach it too. cosmo is
// excluded because gosmopolitan's matchTag now aliases GOOS=cosmo into
// `linux`, and a fat APE still needs proc_cosmo.go's runtime host dispatch
// (a single cosmo binary boots on both Linux and macOS; this file's /proc
// read only works on the former).

// CommPPID reads /proc/<pid>/stat and returns the process's comm and its
// parent PID. ok is false if the entry cannot be read or parsed.
func CommPPID(pid int) (comm string, ppid int, ok bool) { return commPPIDProc(pid) }
