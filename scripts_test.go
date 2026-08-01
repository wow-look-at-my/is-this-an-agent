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
// shell. They are GENERATED from the roster and scripts/engine.sh, so what
// needs checking is not that two hand-maintained copies still agree -- nothing
// is hand-maintained -- but that the committed scripts are what the generator
// produces today, and that they actually behave.

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

func TestGeneratedScriptsAreUpToDate(t *testing.T) {
	// The one check that replaces every hand-maintained-copy check: an agent
	// added to the roster, a marker corrected, or an engine edit that was not
	// regenerated all land here, as a diff.
	for name, want := range GeneratedScripts() {
		path := filepath.Join(scriptsDir, name)
		got, err := os.ReadFile(path)
		require.NoError(t, err, "%s is missing -- run: go run ./cmd/gen-scripts", path)
		assert.Equal(t, want, string(got),
			"%s is stale -- run: go run ./cmd/gen-scripts", path)
	}
}

func TestGeneratedScriptsAreValidShell(t *testing.T) {
	// `sh -n` parses without executing: a broken generator template must fail
	// here, not the first time somebody runs the script.
	for name := range GeneratedScripts() {
		path := filepath.Join(scriptsDir, name)
		out, err := exec.Command("/bin/sh", "-n", path).CombinedOutput()
		assert.NoError(t, err, "%s is not valid sh: %s", path, out)
	}
	out, err := exec.Command("/bin/sh", "-n", filepath.Join(scriptsDir, "engine.sh")).CombinedOutput()
	assert.NoError(t, err, "engine.sh is not valid sh: %s", out)
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
