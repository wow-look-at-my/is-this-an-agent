package agent

// The shell scripts are GENERATED from this package: the roster supplies each
// agent's markers, scripts/engine.sh supplies the detection engine, and the
// two are combined into one standalone script per agent.
//
// They are generated rather than written because they must be standalone --
// copy one anywhere and run it, with nothing to source -- which means the
// engine is duplicated across six files. Duplication a human maintains rots;
// duplication a generator produces and a test re-derives cannot. And with the
// markers coming from the roster, an agent added in Go cannot ship with a
// script that does not know about it.
//
// Regenerate with `go run ./cmd/gen-scripts`; TestGeneratedScriptsAreUpToDate
// fails the build when a committed script differs from what this produces.

import (
	_ "embed"
	"fmt"
	"strings"
)

// engine is the shared detection engine every generated script embeds
// verbatim. It is a real file so it can be read, syntax-checked and edited as
// shell rather than as a Go string literal.
//
//go:embed scripts/engine.sh
var engine string

// ScriptName is the file name of the script for an agent ID, or of the
// any-agent script when id is empty.
func ScriptName(id string) string {
	if id == "" {
		return "is-this-an-agent.sh"
	}
	return "is-this-" + id + ".sh"
}

// GeneratedScripts renders every script, keyed by file name: one per roster
// agent plus the any-agent script.
func GeneratedScripts() map[string]string {
	out := make(map[string]string, len(roster)+1)
	for _, a := range roster {
		out[ScriptName(a.ID)] = RenderScript(a)
	}
	out[ScriptName("")] = RenderRosterScript()
	return out
}

// RenderScript renders the standalone script for one agent.
func RenderScript(a Agent) string {
	var b strings.Builder
	b.WriteString(header(headerText{
		Script: ScriptName(a.ID),
		Title:  fmt.Sprintf("%s -- is this shell running under %s?", ScriptName(a.ID), a.Name),
		What:   "running under " + a.Name,
		Usage:  fmt.Sprintf("Report whether this shell is running under %s.", a.Name),
		Extra:  scriptNotes[a.ID],
	}))
	fmt.Fprintf(&b, "AGENT_NAME='%s'\n", a.Name)
	fmt.Fprintf(&b, "AGENT_ENV_VARS='%s'\n", strings.Join(a.EnvVars, " "))
	fmt.Fprintf(&b, "AGENT_PROCS='%s'\n", strings.Join(a.Procs, " "))
	fmt.Fprintf(&b, "AGENT_PID_VARS='%s'\n\n", strings.Join(a.PIDVars, " "))
	b.WriteString(engine)
	b.WriteString(`
if _iaa_detect; then
    [ "$quiet" -eq 1 ] || printf '%s\n' "$AGENT_NAME"
    exit 0
fi
exit 1
`)
	return b.String()
}

// RenderRosterScript renders the any-agent script: the same engine, fed each
// roster record in turn.
func RenderRosterScript() string {
	names := make([]string, 0, len(roster))
	records := make([]string, 0, len(roster))
	for _, a := range roster {
		names = append(names, a.Name)
		records = append(records, strings.Join([]string{
			a.ID, a.Name,
			strings.Join(a.EnvVars, " "),
			strings.Join(a.Procs, " "),
			strings.Join(a.PIDVars, " "),
		}, "|"))
	}

	var b strings.Builder
	b.WriteString(header(headerText{
		Script: ScriptName(""),
		Title:  ScriptName("") + " -- is this shell running under an AI coding agent?",
		What:   "running under any known agent",
		Usage: "Report whether this shell is running under an AI coding agent.\n" +
			"On success the agent's name is printed: " + strings.Join(names, ", ") + ".",
	}))
	fmt.Fprintf(&b, "# One record per agent: id|Name|env markers|process prefixes|pid variables.\nAGENT_ROSTER='%s'\n\n", strings.Join(records, "\n"))
	b.WriteString(engine)
	b.WriteString(`
# Each record is fed to the same engine in turn: the first agent that matches
# wins, and the roster order is the tie-break. Iterating with IFS set to a
# newline keeps this in THIS shell -- piping the roster into a ` + "`while read`" + `
# loop would run it in a subshell, where a match could not be reported back
# and an unmatched loop exits 0 (the status of its own trailing ` + "`if`" + `).
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
`)
	return b.String()
}

// scriptNotes are per-agent remarks worth carrying into that agent's script,
// where someone reading it in isolation will see them.
var scriptNotes = map[string]string{
	"codex": `# Codex exports no "you are under Codex" marker. What it does export is its
# sandbox state -- CODEX_SANDBOX=seatbelt under the macOS sandbox,
# CODEX_SANDBOX_NETWORK_DISABLED=1 for shell-tool commands whose network is cut
# -- so a Codex run with no sandbox and network allowed is recognized by its
# process ancestry alone.
`,
	"opencode": `# opencode also names its own process in OPENCODE_PID, which is what catches it
# when it runs from a JS runtime and the process name is the runtime's.
`,
}

// headerText is what varies between the scripts' headers.
type headerText struct {
	Script string
	Title  string
	What   string
	Usage  string
	Extra  string
}

// header renders everything above a script's roster data: the doc comment,
// usage, and argument parsing.
func header(h headerText) string {
	return fmt.Sprintf(`#!/bin/sh
# %s
#
# GENERATED by ../cmd/gen-scripts from agent.go and engine.sh -- do not edit.
#
# Exit status: 0 when %s, 1 when not. With -q it prints nothing, so it
# reads well in a conditional:
#
#     if %s -q; then ...; fi
#
# Detection is advisory, never a security boundary: any process can set the
# markers, and an agent can be renamed out of the list. Part of
# github.com/wow-look-at-my/is-this-an-agent, whose Go package answers the same
# question with the same data.

set -u

usage() {
    cat <<EOF
usage: %s [-q] [-h]

%s

  -q  quiet: print nothing, report only through the exit status
  -h  this help
EOF
}

quiet=0
for arg in "$@"; do
    case $arg in
        -q|--quiet) quiet=1 ;;
        -h|--help) usage; exit 0 ;;
        *) printf '%%s: unknown option: %%s\n' "%s" "$arg" >&2; usage >&2; exit 2 ;;
    esac
done

%s`, h.Title, h.What, h.Script, h.Script, h.Usage, h.Script, h.Extra)
}
