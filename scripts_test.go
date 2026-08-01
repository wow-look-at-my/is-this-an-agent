package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The scripts answer the same question as this package for callers that are a
// shell, so they carry a copy of the roster's data. These tests are what keep
// the copy true: markers that drift are markers that quietly stop detecting an
// agent, in the one place nobody thinks to look.

const scriptsDir = "scripts"

func scriptPath(id string) string {
	if id == "" {
		return filepath.Join(scriptsDir, "is-this-an-agent.sh")
	}
	return filepath.Join(scriptsDir, "is-this-"+id+".sh")
}

// runScript runs a script with ONLY the given environment (plus PATH, without
// which the engine has no ps/awk), and returns its stdout and exit code.
func runScript(t *testing.T, path string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", append([]string{path}, args...)...)
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH")}, env...)
	out, err := cmd.Output()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "running %s", path)
		code = exitErr.ExitCode()
	}
	return strings.TrimSpace(string(out)), code
}

func TestEveryAgentHasAScript(t *testing.T) {
	for _, a := range Roster() {
		assert.FileExists(t, scriptPath(a.ID), "%s is on the roster with no script", a.ID)
	}
	assert.FileExists(t, scriptPath(""))

	// And nothing extra: a script for an agent the package does not know is a
	// script nothing keeps in step.
	entries, err := os.ReadDir(scriptsDir)
	require.NoError(t, err)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "is-this-") || !strings.HasSuffix(name, ".sh") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "is-this-"), ".sh")
		if id == "an-agent" {
			continue
		}
		_, known := ByID(id)
		assert.True(t, known, "scripts/%s has no agent %q on the roster", name, id)
	}
}

func TestScriptMarkersMatchTheRoster(t *testing.T) {
	// The per-agent scripts declare their markers as shell variables. Those
	// declarations ARE the roster data, restated -- so they must match it
	// exactly, in content and order.
	for _, a := range Roster() {
		t.Run(a.ID, func(t *testing.T) {
			body, err := os.ReadFile(scriptPath(a.ID))
			require.NoError(t, err)
			src := string(body)

			assert.Equal(t, a.Name, shellVar(t, src, "AGENT_NAME"))
			assert.Equal(t, strings.Join(a.EnvVars, " "), shellVar(t, src, "AGENT_ENV_VARS"))
			assert.Equal(t, strings.Join(a.Procs, " "), shellVar(t, src, "AGENT_PROCS"))
			assert.Equal(t, strings.Join(a.PIDVars, " "), shellVar(t, src, "AGENT_PID_VARS"))
		})
	}
}

func TestRosterScriptMatchesTheRoster(t *testing.T) {
	body, err := os.ReadFile(scriptPath(""))
	require.NoError(t, err)
	records := shellVar(t, string(body), "AGENT_ROSTER")

	var want []string
	for _, a := range Roster() {
		want = append(want, strings.Join([]string{
			a.ID, a.Name,
			strings.Join(a.EnvVars, " "),
			strings.Join(a.Procs, " "),
			strings.Join(a.PIDVars, " "),
		}, "|"))
	}
	assert.Equal(t, strings.Join(want, "\n"), records)
}

// shellVar extracts a single-quoted shell assignment (NAME='value'), which is
// the only form these scripts use for roster data.
func shellVar(t *testing.T, src, name string) string {
	t.Helper()
	marker := "\n" + name + "='"
	i := strings.Index("\n"+src, marker)
	require.GreaterOrEqual(t, i, 0, "%s is not assigned in the script", name)
	rest := ("\n" + src)[i+len(marker):]
	end := strings.Index(rest, "'")
	require.GreaterOrEqual(t, end, 0, "%s assignment is not closed", name)
	return rest[:end]
}

func TestScriptEnginesAreIdentical(t *testing.T) {
	// The scripts are standalone on purpose -- copy one anywhere and it runs
	// -- so the engine is duplicated rather than sourced. Duplication is only
	// safe while the copies are identical, which is what this checks.
	const start = "# --- shared detection engine "
	const end = "# --- end shared detection engine "

	var reference, referenceName string
	for _, a := range append(Roster(), Agent{ID: ""}) {
		path := scriptPath(a.ID)
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		src := string(body)

		i := strings.Index(src, start)
		j := strings.Index(src, end)
		require.GreaterOrEqual(t, i, 0, "%s has no engine block", path)
		require.Greater(t, j, i, "%s has no engine terminator", path)
		block := src[i:j]

		if reference == "" {
			reference, referenceName = block, path
			continue
		}
		assert.Equal(t, reference, block,
			"the detection engine in %s has drifted from %s", path, referenceName)
	}
}

func TestScriptsDetectTheirMarker(t *testing.T) {
	// A positive is unambiguous whatever the machine's process tree looks
	// like: with the marker set, the script must say so.
	for _, a := range Roster() {
		for _, marker := range a.EnvVars {
			t.Run(a.ID+"/"+marker, func(t *testing.T) {
				out, code := runScript(t, scriptPath(a.ID), []string{marker + "=1"})
				assert.Equal(t, 0, code, "%s=1 must be detected", marker)
				assert.Equal(t, a.Name, out)

				out, code = runScript(t, scriptPath(""), []string{marker + "=1"})
				assert.Equal(t, 0, code)
				if _, underAgent := ProcessAncestor(); !underAgent {
					// Under a real agent, ancestry legitimately names that
					// agent instead -- so only assert the name when there is
					// no agent above this test.
					assert.Equal(t, a.Name, out)
				}
			})
		}
	}
}

func TestScriptsIgnoreOtherAgentsMarkers(t *testing.T) {
	if _, underAgent := ProcessAncestor(); underAgent {
		t.Skip("this test process is under an agent; its ancestry would answer for every script")
	}
	for _, a := range Roster() {
		for _, other := range Roster() {
			if other.ID == a.ID {
				continue
			}
			_, code := runScript(t, scriptPath(a.ID), []string{other.EnvVars[0] + "=1"})
			assert.Equal(t, 1, code, "is-this-%s.sh must not fire on %s", a.ID, other.EnvVars[0])
		}
	}
}

func TestScriptsWithNoAgent(t *testing.T) {
	if _, underAgent := ProcessAncestor(); underAgent {
		t.Skip("this test process is under an agent; a clean-env run still has that ancestry")
	}
	for _, a := range append(Roster(), Agent{ID: ""}) {
		out, code := runScript(t, scriptPath(a.ID), nil)
		assert.Equal(t, 1, code, "%s must report nothing when no agent is present", scriptPath(a.ID))
		assert.Empty(t, out)
	}
}

func TestScriptMarkerSetToZeroDoesNotCount(t *testing.T) {
	if _, underAgent := ProcessAncestor(); underAgent {
		t.Skip("this test process is under an agent; its ancestry would answer instead")
	}
	_, code := runScript(t, scriptPath("claude"), []string{"CLAUDECODE=0"})
	assert.Equal(t, 1, code, "an explicit CLAUDECODE=0 says 'not under it'")
}

func TestScriptArgumentSurface(t *testing.T) {
	for _, a := range append(Roster(), Agent{ID: ""}) {
		path := scriptPath(a.ID)

		out, code := runScript(t, path, []string{"CLAUDECODE=1", "GROK_AGENT=1", "CODEX_SANDBOX=seatbelt", "GEMINI_CLI=1", "OPENCODE=1"}, "-q")
		assert.Equal(t, 0, code, "%s: -q keeps the exit status", path)
		assert.Empty(t, out, "%s: -q must print nothing", path)

		out, code = runScript(t, path, nil, "-h")
		assert.Equal(t, 0, code, "%s: -h exits 0", path)
		assert.Contains(t, out, "usage:", path)

		_, code = runScript(t, path, nil, "--nope")
		assert.Equal(t, 2, code, "%s: an unknown option is a usage error, not a verdict", path)
	}
}

func TestScriptsAreExecutableAndPortable(t *testing.T) {
	for _, a := range append(Roster(), Agent{ID: ""}) {
		path := scriptPath(a.ID)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&0o111, "%s must be executable", path)

		body, err := os.ReadFile(path)
		require.NoError(t, err)
		src := string(body)
		assert.True(t, strings.HasPrefix(src, "#!/bin/sh\n"), "%s must be a POSIX sh script", path)

		// The scripts run under whatever /bin/sh a machine has -- dash, ash,
		// busybox -- so the usual bash-only spellings must stay out.
		for _, bashism := range []string{"[[", "==", "function ", "${BASH", "local "} {
			assert.NotContains(t, src, bashism, "%s: %q is not POSIX sh", path, bashism)
		}
	}
}
