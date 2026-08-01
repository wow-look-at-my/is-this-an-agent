// Package agent answers one question: is this process running under an AI
// coding agent, and which one?
//
// It exists because that question keeps getting answered ad hoc, one CLAUDECODE
// check at a time, in whatever tool needs it -- and every such check is wrong
// for the agent it did not know about. The roster below is the shared answer:
// each agent's environment markers, the process names it runs under, and the
// variables it uses to name its own PID.
//
//	if a, ok := agent.Detect(); ok {
//	        fmt.Printf("running under %s\n", a.Name)
//	}
//
// Detection is by process ancestry first and environment marker second.
// Ancestry is the stronger signal: markers are inherited by every descendant,
// so a marker survives into processes the agent did not spawn (a daemon started
// from an agent session keeps it forever), while an ancestor named `claude` is
// this process's actual lineage. The env fallback is what keeps detection
// working where the ancestry walk cannot run -- a platform without /proc, an
// agent whose launcher is renamed, a process re-parented to init.
//
// Neither signal is a security boundary. Everything here is advisory: any
// process can set CLAUDECODE=1, and an agent can be renamed out of the roster.
// The point is to let a tool behave sensibly under an agent, not to prove
// anything to an adversary.
//
// The shell scripts in scripts/ answer the same question for a shell, one
// standalone script per agent, using the same markers -- kept in step with this
// roster by a test.
package agent

import (
	"os"
	"strconv"
	"strings"
)

// Agent is one AI coding agent go-toolchain and friends can run underneath.
type Agent struct {
	// ID is the short, stable identifier: the suffix of the matching shell
	// script (is-this-<id>.sh) and what Is() takes.
	ID string

	// Name is the human-readable name, for messages ("running under grok
	// build").
	Name string

	// EnvVars are markers the agent exports into every child's environment.
	// A variable that is set, non-empty and not "0" identifies the agent.
	EnvVars []string

	// Procs are process-name prefixes of the agent process itself, matched
	// against /proc comm -- which the kernel truncates to 15 bytes, hence
	// prefixes rather than exact names.
	Procs []string

	// PIDVars are variables holding the agent process's own PID. They cover
	// an agent running from a JS or other runtime, where the process name is
	// the runtime's rather than the agent's.
	PIDVars []string
}

// roster is every agent this package knows. Adding one here without adding
// its scripts/is-this-<id>.sh is a test failure, and vice versa.
var roster = []Agent{
	{
		ID:      "claude",
		Name:    "Claude",
		EnvVars: []string{"CLAUDECODE"},
		Procs:   []string{"claude"},
	},
	{
		ID:      "grok",
		Name:    "grok build",
		EnvVars: []string{"GROK_AGENT"},
		Procs:   []string{"grok", "xai-grok-pager"},
	},
	{
		ID:   "codex",
		Name: "Codex",
		// Codex has no "you are under Codex" marker. What it does export is
		// its sandbox state: CODEX_SANDBOX=seatbelt under the macOS sandbox,
		// and CODEX_SANDBOX_NETWORK_DISABLED=1 for shell-tool commands whose
		// network is cut. Both are set BY Codex on its own children, so
		// either one identifies it -- but neither is guaranteed (a run with
		// no sandbox and network allowed sets nothing), which is exactly why
		// ancestry is checked first.
		EnvVars: []string{"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"},
		Procs:   []string{"codex"},
	},
	{
		ID:      "gemini",
		Name:    "Gemini CLI",
		EnvVars: []string{"GEMINI_CLI"},
		Procs:   []string{"gemini"},
	},
	{
		ID:      "opencode",
		Name:    "opencode",
		EnvVars: []string{"OPENCODE"},
		Procs:   []string{"opencode"},
		PIDVars: []string{"OPENCODE_PID"},
	},
}

// Roster returns every known agent, in a fixed order. The slice is a copy:
// callers cannot mutate the roster.
func Roster() []Agent {
	out := make([]Agent, len(roster))
	copy(out, roster)
	return out
}

// ByID returns the agent with the given ID.
func ByID(id string) (Agent, bool) {
	for _, a := range roster {
		if a.ID == id {
			return a, true
		}
	}
	return Agent{}, false
}

// Detect returns the agent this process is running under: by process ancestry
// first, then by environment marker. See the package comment for why that
// order.
func Detect() (Agent, bool) {
	if a, ok := ProcessAncestor(); ok {
		return a, true
	}
	return FromEnv()
}

// Is reports whether this process is running under the agent with the given
// ID. An unknown ID is false, never a panic -- callers routinely pass a
// string from a flag or an argv[0].
func Is(id string) bool {
	a, ok := Detect()
	return ok && a.ID == id
}

// FromEnv returns the agent whose environment marker is set. A marker counts
// when it is present, non-empty and not "0", so an explicit MARKER=0 reads as
// "not under it" rather than as the agent.
func FromEnv() (Agent, bool) {
	for _, a := range roster {
		for _, v := range a.EnvVars {
			if val := os.Getenv(v); val != "" && val != "0" {
				return a, true
			}
		}
	}
	return Agent{}, false
}

// ForProcess returns the agent whose process-name prefix matches comm (a
// /proc comm value, kernel-truncated to 15 bytes).
func ForProcess(comm string) (Agent, bool) {
	for _, a := range roster {
		for _, p := range a.Procs {
			if strings.HasPrefix(comm, p) {
				return a, true
			}
		}
	}
	return Agent{}, false
}

// IsPID reports whether pid is an agent process that named itself in the
// environment (opencode exports OPENCODE_PID).
func IsPID(pid int) bool {
	_, ok := AgentForPID(pid)
	return ok
}

// AgentForPID returns the agent that named pid as its own process in the
// environment.
func AgentForPID(pid int) (Agent, bool) {
	for _, a := range roster {
		for _, v := range a.PIDVars {
			if s := os.Getenv(v); s != "" {
				if p, err := strconv.Atoi(s); err == nil && p == pid {
					return a, true
				}
			}
		}
	}
	return Agent{}, false
}
