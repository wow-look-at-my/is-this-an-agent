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
- `proc_darwin.go` (`darwin`) — the sysctl(KERN_PROC) lookup `CommPPID` plus
  the real entry points, for native darwin builds (no /proc there).
- `proc_other.go` (`!linux && !cosmo && !darwin`) — stubs for platforms with
  no process-tree lookup at all (windows); detection there is by environment
  marker. They answer `false`, never a guess: callers use them to GRANT an
  allowance.
- `capture.go` — `IsCapturePath`: the one redirect that does not hide output
  (the harness's own transcript capture). Claude-only, because it is the only
  agent whose capture path is identifiable.
- `scripts/engine.sh` — the shared POSIX sh detection engine, and the only
  copy anyone edits.
- `scripts/is-this-*.sh` — **generated** (`go run ./cmd/gen-scripts`), one
  standalone script per agent plus `is-this-an-agent.sh` for any. Exit 0 =
  detected. Standalone means nothing to source, which means the engine is
  embedded in each — duplication a generator produces and a test re-derives,
  never duplication a human maintains.
- `scriptgen.go` + `cmd/gen-scripts` — the generator: roster + engine.sh in,
  scripts out (`GeneratedScripts`, `RenderScript`, `RenderRosterScript`).
  `scripts_test.go` regenerates and compares, so a stale script fails CI; it
  also parses each script with `sh -n`, runs them, and bans bashisms.

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
2. Run `go run ./cmd/gen-scripts` — the script for it is generated, along with
   its entry in `is-this-an-agent.sh`. Never write one by hand.
3. Run `go-toolchain`. A forgotten regeneration is a failing test.
