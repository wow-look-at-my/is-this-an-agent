package agent

import (
	"os"
	"strings"
)

// IsCapturePath reports whether path is an agent harness's own capture file --
// the file it redirects a command's stdout to and streams verbatim into its
// transcript.
//
// It exists for one specific decision: a tool that refuses to run when its
// output is being hidden must not refuse when the redirect IS how the agent
// reads it. Every agent-introduced redirect (`> out.log`, `> /dev/null`) is
// something else and stays refused.
//
// Only Claude Code is recognized today, because it is the only agent on the
// roster whose capture path is identifiable: it embeds the session id
// (CLAUDE_CODE_SESSION_ID) and ends in ".output" under a ".../tasks/"
// directory. The session id is the strong signal; the ".output" + "claude"
// structural match is a fallback so a minor change to the harness's path
// scheme cannot wedge a guard into refusing every ordinary run.
func IsCapturePath(path string) bool {
	if sid := os.Getenv("CLAUDE_CODE_SESSION_ID"); sid != "" && strings.Contains(path, sid) {
		return true
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".output") && strings.Contains(lower, "claude")
}
