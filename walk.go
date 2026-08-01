package agent

// The parent-chain walk, with the chain supplied by the caller. It lives here,
// free of build tags, because only the /proc lookup is platform-specific: the
// matching rules are the same everywhere, and a walk you can hand a chain to
// is the only kind you can test without owning the machine's process tree.

// Lookup returns a PID's comm (the executable base name, as /proc reports it,
// truncated to 15 bytes by the kernel) and its parent PID. ok is false when
// the process cannot be inspected, which ends the walk.
type Lookup func(pid int) (comm string, ppid int, ok bool)

// maxHops bounds every walk. A real chain is a handful of hops; the bound is
// defensive against a pid-reuse race turning the walk into a cycle.
const maxHops = 64

// AncestorAgent walks from start up the parent chain and returns the first
// agent it finds, by process name or by a PID an agent named as its own.
func AncestorAgent(start int, lookup Lookup) (Agent, bool) {
	pid := start
	for hops := 0; pid > 1 && hops < maxHops; hops++ {
		comm, ppid, ok := lookup(pid)
		if !ok {
			return Agent{}, false
		}
		if a, ok := ForProcess(comm); ok {
			return a, true
		}
		if a, ok := AgentForPID(pid); ok {
			return a, true
		}
		pid = ppid
	}
	return Agent{}, false
}

// AncestorPID reports whether target appears in the chain starting at start.
func AncestorPID(start, target int, lookup Lookup) bool {
	pid := start
	for hops := 0; pid > 1 && hops < maxHops; hops++ {
		if pid == target {
			return true
		}
		_, ppid, ok := lookup(pid)
		if !ok {
			return false
		}
		pid = ppid
	}
	return pid == target
}

// PipeReader reports whether the process (comm, pid) reading one of our pipes
// is an agent capturing our output, rather than a filter in a shell pipeline
// or the shell of a `$(...)` capture.
//
// This is the question a tool asks when it wants to tell "the agent is reading
// my stdout, so it will be shown" from "my output is being swallowed by
// `| head`". The distinguishing property is ancestry: an agent capturing a
// command's output is that command's ancestor, while a pipeline filter is a
// sibling and a `$(...)` reader is a shell.
func PipeReader(comm string, pid, start int, lookup Lookup) bool {
	if !AncestorPID(start, pid, lookup) {
		return false
	}
	if _, ok := ForProcess(comm); ok {
		return true
	}
	return IsPID(pid)
}
