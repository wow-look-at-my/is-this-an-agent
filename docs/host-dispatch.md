# Choosing a process lookup by host

A GOOS=cosmo binary is a gosmopolitan fat APE: one file that boots natively on
Linux, macOS and Windows. On Linux and macOS it reports `runtime.GOOS ==
"cosmo"` either way, so a build tag cannot pick its process lookup — the host
is only known at runtime. (On Windows it runs its embedded native
GOOS=windows payload, which compiles the non-cosmo files instead.)

`HostOS()` answers that question, and `lookupForHost` turns the answer into
the lookup: `/proc` on Linux, `ps(1)` on macOS.

## Why macOS gets ps and not the sysctl

`proc_darwin.go` reads `kinfo_proc` through
`unix.SysctlKinfoProc("kern.proc.pid", pid)`, and that is the right lookup for
a binary compiled FOR darwin. An APE cannot use it, in either direction:

- `golang.org/x/sys/unix` has no cosmo port. `SysctlKinfoProc` is declared in
  `syscall_darwin.go`; under GOOS=cosmo the package selects no implementation
  for it and the build fails with `undefined: unix.SysctlKinfoProc`.
- A raw sysctl is not emulated. The fork's darwin syscall dispatcher
  (`src/internal/runtime/syscall/cosmo/syscall_cosmo_arm64.go`) handles a
  fixed set of Linux syscall numbers and answers ENOSYS for everything else.
  Neither `sysctl` nor `_sysctl` is in that set, and `syscall.Sysctl` — the
  stdlib BSD wrapper — is not compiled for cosmo either.

`ps -o ppid=,ucomm= -p <pid>` reads the same `kinfo_proc` through the OS's own
tool. `ucomm` is `P_comm`, the MAXCOMLEN-truncated accounting name, which is
the same string `/proc` reports as `comm` — so a roster prefix matches
identically whichever lookup answered. `os/exec` works on an APE on macOS
(gosmopolitan CI proves fork, pipes, execve, wait4 and LookPath there), so
this is reachable where the sysctl is not.

The cost is a fork per hop. A walk is a handful of hops and runs once at
startup.

## The probe, and what a sandbox does to it

`hostFromEvidence` weighs signals in order of how much a sandbox can interfere
with them. **No rung ends in a default.** Evidence that identifies nothing
yields `""`.

1. **What the runtime booted with.** The cosmo runtime records the host at
   `rt0` and dispatches every syscall on it, so it is fact rather than
   inference and cannot be denied, refused or raced. It is reachable from user
   code when the toolchain exports it; `runtimeHostOS()` in `host_cosmo.go` is
   the seam, and returns `""` on a toolchain that predates the export.
2. **`uname(2)`.** A syscall, so no filesystem policy changes its answer. On a
   Linux host the raw Linux-numbered call passes straight through and names
   Linux. On a macOS host the darwin dispatcher has no case for it and returns
   ENOSYS — so its silence is itself evidence, used by rung 5.
3. **`/proc/self` present** ⟹ linux.
4. **`/System/Library/CoreServices` present** ⟹ darwin.
5. **`/proc/self` definitely absent** ⟹ darwin. Under cosmo the host is Linux
   or macOS, and procfs is on every Linux.

Rungs 3–5 turn on the distinction `statPath` makes between a path that is
*absent* (ENOENT) and one whose stat was *refused* (EPERM, EACCES). A refusal
is evidence of nothing. Reading it as absence is what makes a probe conclude
"linux" on a Mac: that is the failure mode of a probe that ends in
`return "linux"`, and rung 5 is why this one does not need such an ending.

Worked cases:

| host | uname | `/proc/self` | CoreServices | answer |
|---|---|---|---|---|
| Linux | `Linux` | present | absent | linux |
| macOS | ENOSYS | absent | present | darwin |
| macOS, sandbox hides `/System/Library` | ENOSYS | absent | denied | **darwin** |
| Linux, uname blocked | fails | present | denied | linux |
| every signal refused | fails | denied | denied | **`""`** |

## What `""` means

`HostOS()` returning `""` is a real answer: *no signal identified this host*.
It is never a synonym for linux.

`lookupForHost` handles it without guessing — it tries both lookups, `/proc`
first. Neither can answer wrongly on the other's host, because
`/proc/<pid>/stat` does not exist on macOS and `ps` reports the same two
fields on both. So a blind probe costs one failed file read, not a wrong
answer, and `CommPPID` still resolves.

A caller that needs to know it was blind can ask `HostOS()` directly.

## Testing something no runner can run

The configuration that matters — an APE on a macOS host — is one no CI runner
in this org is. What CI does cover:

- **The decision**, on every platform: `hostFromEvidence` and `lookupForHost`
  take their inputs as data, so each host above, sandboxed ones included, is a
  case in `host_test.go`.
- **The ps lookup**, on linux and darwin: `procps_test.go` asserts it agrees
  with the platform's native lookup, field for field, on a live process.
- **The APE itself**, in the `cosmo-ape-check` CI job: it installs the
  gosmopolitan toolchain, builds the package for GOOS=cosmo and RUNS the
  suite — an APE executes natively on a Linux runner, so that is the real
  binary probing a real host and taking the `/proc` branch.

What remains unproven is the darwin branch executing on real hardware: an APE
on a Mac calling `ps`. Running this package's suite under GOOS=cosmo on a macOS
runner is what would settle it.
