# CLAUDE.md

The one question this repo answers: is this process running under an AI coding
agent, and which one? Two consumers of one roster — a Go package, and a
standalone shell script per agent.

## Build & Test

`go-toolchain` (no arguments) in the repo root. Never bare `go` commands.

## CI

`.github/workflows/ci.yml` runs three jobs: `test` (the toolchain's own
build+test+vet), `cross-compile-check` (darwin/windows build-only smoke),
and `cosmo-ape-check` (installs the gosmopolitan fork and runs the suite
as a real APE). Depth: `docs/CI.md`.

## Layout

- `agent.go` — the **roster** (each agent's `ID`, `Name`, env markers, process
  prefixes, PID variables) and the env/name matching: `Roster`, `ByID`,
  `Detect`, `Is`, `FromEnv`, `ForProcess`, `IsPID`, `AgentForPID`. Adding an
  agent starts here, and is incomplete without `scripts/is-this-<id>.sh`.
- `walk.go` — the parent-chain walk, with the chain supplied by the caller
  (`Lookup`): `AncestorAgent`, `AncestorPID`, `PipeReader`. No build tags —
  only the lookup is platform-specific, and a walk you can hand a chain to is
  the only kind that is testable.
- `proc.go` (`linux || darwin || cosmo`) — the real entry points
  `ProcessAncestor`, `IsAncestorPID`, `IsPipeReader`, built on whichever
  `CommPPID` the platform supplies. One copy, not one per platform.
- `procfs.go` (`linux || cosmo`) — the /proc lookup. The constraint is
  `linux || cosmo`, NOT `linux`: this org's released "linux" binaries are
  GOOS=cosmo APE copies, and a `_linux.go` filename would compile detection out
  of every one of them while the GOOS=linux tests stayed green.
- `procps.go` (`linux || darwin || cosmo`) — the `ps(1)` lookup, which is what
  an APE uses on a macOS host: `x/sys/unix` has no cosmo port and the fork's
  darwin dispatcher emulates no sysctl, so `ps` is the only reachable reader of
  the same `kinfo_proc`. Compiled on linux and darwin too, unused there outside
  tests — that is how the one lookup no CI runner executes in its real
  configuration gets checked against the two that can.
- `proc_linux.go`, `proc_darwin.go`, `proc_cosmo.go` — one `CommPPID` each:
  /proc, sysctl(KERN_PROC), and the host dispatch between /proc and `ps`.
- `host.go` — `HostOS()`/`HostSource()`'s decision, free of build tags:
  `hostFromEvidence` and `lookupForHost` take their inputs as data, so a Mac in
  a sandbox that denies the probe paths is a test case on every platform. Every
  answer names the signal that produced it, so one log line separates "read the
  machine" from "read nothing". `host_cosmo.go` gathers the evidence (uname,
  then path probes); `host_other.go` (`!cosmo`) is `runtime.GOOS`, because every
  other build runs on what it was compiled for. Depth: `docs/host-dispatch.md`,
  which also names go-toolchain's `smoke-macos` job as the integration prover
  for the darwin branch.
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
  Every lookup reports that same accounting name — sysctl's `P_comm` and ps's
  `ucomm` are the same field — so a prefix matches whichever one answered.
- **The lookup is chosen by HOST, not by `runtime.GOOS`.** A cosmo APE is one
  binary that boots on Linux and on macOS, and answers `"cosmo"` on both. An
  unidentified host is `""` and asks both lookups; it is never assumed to be
  linux, which is the failure that made an APE on a Mac resolve nothing.
- **Detection is advisory, never a security boundary.** Anyone can set a
  marker; the roster is about behaving sensibly, not about proof.
- **Every walk is bounded** (`maxHops`): pid reuse must not become a loop.

## Adding an agent

1. Add it to `roster` in `agent.go` with its markers, process prefixes and any
   PID variable, and say in a comment where the markers came from.
2. Run `go run ./cmd/gen-scripts` — the script for it is generated, along with
   its entry in `is-this-an-agent.sh`. Never write one by hand.
3. Run `go-toolchain`. A forgotten regeneration is a failing test.
