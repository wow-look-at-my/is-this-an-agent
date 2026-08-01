# CLAUDE.md

The one question this repo answers: is this process running under an AI coding
agent, and which one? Two consumers of one roster — a Go package, and a
standalone shell script per agent.

## Build & Test

`go-toolchain` (no arguments) in the repo root. Never bare `go` commands.

## Layout

- `agent.go` — the **roster** (each agent's `ID`, `Name`, env markers, process
  prefixes, PID variables) and the env/name matching: `Roster`, `ByID`,
  `Detect`, `Is`, `FromEnv`, `ForProcess`, `IsPID`, `AgentForPID`. Adding an
  agent starts here, and is incomplete without `scripts/is-this-<id>.sh`.
- `walk.go` — the parent-chain walk, with the chain supplied by the caller
  (`Lookup`): `AncestorAgent`, `AncestorPID`, `PipeReader`. No build tags —
  only the /proc lookup is platform-specific, and a walk you can hand a chain
  to is the only kind that is testable.
- `proc.go` (`linux || cosmo`) — the /proc lookup `CommPPID` plus the real
  entry points `ProcessAncestor`, `IsAncestorPID`, `IsPipeReader`. The
  constraint is `linux || cosmo`, NOT `linux`: this org's released "linux"
  binaries are GOOS=cosmo APE copies, and a `_linux.go` filename would compile
  detection out of every one of them while the GOOS=linux tests stayed green.
- `proc_other.go` (`!linux && !cosmo`) — stubs for platforms with no /proc;
  detection there is by environment marker. They answer `false`, never a
  guess: callers use them to GRANT an allowance.
- `capture.go` — `IsCapturePath`: the one redirect that does not hide output
  (the harness's own transcript capture). Claude-only, because it is the only
  agent whose capture path is identifiable.
- `scripts/is-this-*.sh` — one standalone POSIX sh script per agent, plus
  `is-this-an-agent.sh` for any. Exit 0 = detected. They duplicate a shared
  engine block **byte-identically** (standalone means nothing to source);
  `scripts_test.go` fails on drift, on a roster/script mismatch, and on
  bashisms.

## Invariants

- **Ancestry beats environment.** A marker is inherited by every descendant
  forever (a daemon started from an agent session keeps it); an ancestor
  process is the actual lineage. `Detect` checks ancestry first, and so do the
  scripts.
- **A marker set to `0` or empty does not count** — an explicit opt-out must
  not read as the agent it denies.
- **Process names are prefixes**, because /proc comm is truncated to 15 bytes.
- **Detection is advisory, never a security boundary.** Anyone can set a
  marker; the roster is about behaving sensibly, not about proof.
- **Every walk is bounded** (`maxHops`): pid reuse must not become a loop.

## Adding an agent

1. Add it to `roster` in `agent.go` with its markers, process prefixes and any
   PID variable, and say in a comment where the markers came from.
2. Add `scripts/is-this-<id>.sh` — copy an existing one, change only the
   header block; the engine must stay byte-identical.
3. Run `go-toolchain`. The tests check both halves agree.
