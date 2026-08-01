#!/bin/sh
# is-this-an-agent.sh -- is this shell running under an AI coding agent?
#
# Exit status: 0 when running under any known agent, 1 when not. With -q it prints nothing, so it
# reads well in a conditional:
#
#     if is-this-an-agent.sh -q; then ...; fi
#
# Detection is advisory, never a security boundary: any process can set the
# markers, and an agent can be renamed out of the list. Part of
# github.com/wow-look-at-my/is-this-an-agent, whose Go package answers the same
# question with the same data.

set -u

usage() {
    cat <<EOF
usage: is-this-an-agent.sh [-q] [-h]

Report whether this shell is running under an AI coding agent.
On success the agent's name is printed: Claude, grok build, Codex, Gemini CLI, opencode.

  -q  quiet: print nothing, report only through the exit status
  -h  this help
EOF
}

quiet=0
for arg in "$@"; do
    case $arg in
        -q|--quiet) quiet=1 ;;
        -h|--help) usage; exit 0 ;;
        *) printf '%s: unknown option: %s\n' "is-this-an-agent.sh" "$arg" >&2; usage >&2; exit 2 ;;
    esac
done

# One record per agent: id|Name|env markers|process prefixes|pid variables.
# Keep this in step with the Go roster in agent.go (scripts_test.go checks it).
AGENT_ROSTER='claude|Claude|CLAUDECODE|claude|
grok|grok build|GROK_AGENT|grok xai-grok-pager|
codex|Codex|CODEX_SANDBOX CODEX_SANDBOX_NETWORK_DISABLED|codex|
gemini|Gemini CLI|GEMINI_CLI|gemini|
opencode|opencode|OPENCODE|opencode|OPENCODE_PID'

# --- shared detection engine ------------------------------------------------
# BYTE-IDENTICAL in every is-this-*.sh. These scripts are standalone on
# purpose -- you can copy one anywhere and run it, with nothing to source --
# so the engine is duplicated rather than shared, and scripts_test.go fails
# the build if the copies drift.
#
# It reads three variables set above it (space-separated lists, any may be
# empty) and answers with an exit status:
#   AGENT_ENV_VARS  markers the agent exports into every child's environment
#   AGENT_PROCS     process-name prefixes of the agent process itself
#   AGENT_PID_VARS  variables holding the agent process's own PID

# _iaa_comm_ppid PID -- prints "<comm><TAB><ppid>" for PID.
_iaa_comm_ppid() {
    _iaa_q=$1
    if [ -r "/proc/$_iaa_q/stat" ]; then
        _iaa_line=$(cat "/proc/$_iaa_q/stat" 2>/dev/null) || return 1
        # "<pid> (<comm>) <state> <ppid> ...". comm can contain spaces and
        # parentheses, so cut at the FIRST '(' and the LAST ')'.
        _iaa_comm=${_iaa_line#*\(}
        _iaa_comm=${_iaa_comm%\)*}
        _iaa_rest=${_iaa_line##*\)}
        _iaa_ppid=$(printf '%s\n' "$_iaa_rest" | awk '{print $2}')
    else
        # No procfs (macOS, BSD): ps knows both fields.
        _iaa_out=$(ps -o ppid=,comm= -p "$_iaa_q" 2>/dev/null) || return 1
        [ -n "$_iaa_out" ] || return 1
        _iaa_ppid=$(printf '%s\n' "$_iaa_out" | awk '{print $1}')
        _iaa_comm=$(printf '%s\n' "$_iaa_out" | awk '{$1=""; sub(/^[ \t]*/, ""); print}')
        _iaa_comm=${_iaa_comm##*/}
    fi
    case $_iaa_ppid in ''|*[!0-9]*) return 1 ;; esac
    printf '%s\t%s\n' "$_iaa_comm" "$_iaa_ppid"
}

# _iaa_proc_matches COMM -- true when COMM starts with one of AGENT_PROCS.
# The kernel truncates comm to 15 bytes, so these are prefixes, not names.
_iaa_proc_matches() {
    for _iaa_p in $AGENT_PROCS; do
        case $1 in "$_iaa_p"*) return 0 ;; esac
    done
    return 1
}

# _iaa_pid_matches PID -- true when one of AGENT_PID_VARS names PID.
_iaa_pid_matches() {
    for _iaa_v in $AGENT_PID_VARS; do
        eval "_iaa_val=\${$_iaa_v-}"
        [ -n "$_iaa_val" ] && [ "$_iaa_val" = "$1" ] && return 0
    done
    return 1
}

# _iaa_env_marker -- true when one of AGENT_ENV_VARS is set, non-empty and not
# "0", so an explicit MARKER=0 reads as "not under it".
_iaa_env_marker() {
    for _iaa_v in $AGENT_ENV_VARS; do
        eval "_iaa_val=\${$_iaa_v-}"
        [ -n "$_iaa_val" ] && [ "$_iaa_val" != 0 ] && return 0
    done
    return 1
}

# _iaa_ancestor -- true when an ancestor process is the agent. Walks the
# parent chain, bounded (a pid-reuse race must not become a loop).
_iaa_ancestor() {
    _iaa_walk=$PPID
    _iaa_hops=0
    while [ "$_iaa_walk" -gt 1 ] && [ "$_iaa_hops" -lt 64 ]; do
        _iaa_info=$(_iaa_comm_ppid "$_iaa_walk") || return 1
        _iaa_c=${_iaa_info%%	*}
        _iaa_n=${_iaa_info##*	}
        _iaa_proc_matches "$_iaa_c" && return 0
        _iaa_pid_matches "$_iaa_walk" && return 0
        _iaa_walk=$_iaa_n
        _iaa_hops=$((_iaa_hops + 1))
    done
    return 1
}

# _iaa_detect -- ancestry first, environment marker second. Ancestry is the
# stronger signal: a marker is inherited by every descendant forever, while an
# ancestor named `claude` is this process's actual lineage.
_iaa_detect() {
    _iaa_ancestor && return 0
    _iaa_env_marker && return 0
    return 1
}
# --- end shared detection engine --------------------------------------------

# Each record is fed to the same engine in turn: the first agent that matches
# wins, and the roster order is the tie-break. Iterating with IFS set to a
# newline keeps this in THIS shell -- piping the roster into a `while read`
# loop would run it in a subshell, where a match could not be reported back
# and an unmatched loop exits 0 (the status of its own trailing `if`).
_iaa_found=''
_iaa_ifs=$IFS
IFS='
'
for _iaa_rec in $AGENT_ROSTER; do
    IFS=$_iaa_ifs
    _iaa_rest=$_iaa_rec
    _iaa_rest=${_iaa_rest#*|}
    _iaa_name=${_iaa_rest%%|*}
    _iaa_rest=${_iaa_rest#*|}
    AGENT_ENV_VARS=${_iaa_rest%%|*}
    _iaa_rest=${_iaa_rest#*|}
    AGENT_PROCS=${_iaa_rest%%|*}
    _iaa_rest=${_iaa_rest#*|}
    AGENT_PID_VARS=$_iaa_rest
    if _iaa_detect; then
        _iaa_found=$_iaa_name
        break
    fi
    IFS='
'
done
IFS=$_iaa_ifs

if [ -n "$_iaa_found" ]; then
    [ "$quiet" -eq 1 ] || printf '%s\n' "$_iaa_found"
    exit 0
fi
exit 1
