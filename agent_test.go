package agent

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wow-look-at-my/go-containers/set"
)

// clearMarkers unsets every roster marker (and PID variable) for the duration
// of a test, so a run under a real agent -- which is where this package tends
// to be developed -- cannot make an env-detection test pass or fail for the
// wrong reason.
func clearMarkers(t *testing.T) {
	t.Helper()
	for _, a := range roster {
		for _, v := range a.EnvVars {
			t.Setenv(v, "")
		}
		for _, v := range a.PIDVars {
			t.Setenv(v, "")
		}
	}
}

func TestRosterIsWellFormed(t *testing.T) {
	seenID := set.New[string]()
	seenEnv := map[string]string{}
	for _, a := range roster {
		assert.NotEmpty(t, a.ID, "every agent needs an ID")
		assert.NotEmpty(t, a.Name, "every agent needs a Name")
		assert.NotEmpty(t, a.EnvVars, "%s: an agent with no env marker cannot be detected off a process tree", a.ID)
		assert.NotEmpty(t, a.Procs, "%s: an agent with no process name cannot be detected by ancestry", a.ID)
		assert.False(t, seenID.Contains(a.ID), "duplicate agent id %q", a.ID)
		seenID.Add(a.ID)

		// A marker claimed by two agents makes detection order-dependent and
		// the answer arbitrary.
		for _, v := range a.EnvVars {
			assert.Empty(t, seenEnv[v], "env marker %q is claimed by both %q and %q", v, seenEnv[v], a.ID)
			seenEnv[v] = a.ID
		}
	}
}

func TestRosterIsACopy(t *testing.T) {
	got := Roster()
	require.NotEmpty(t, got)
	got[0].Name = "mutated"
	assert.NotEqual(t, "mutated", roster[0].Name, "Roster must not hand out the package's own slice")
}

func TestByID(t *testing.T) {
	a, ok := ByID("claude")
	require.True(t, ok)
	assert.Equal(t, "Claude", a.Name)

	_, ok = ByID("nope")
	assert.False(t, ok)
}

func TestFromEnv(t *testing.T) {
	t.Run("no markers", func(t *testing.T) {
		clearMarkers(t)
		_, ok := FromEnv()
		assert.False(t, ok)
	})

	for _, a := range Roster() {
		t.Run(a.ID, func(t *testing.T) {
			clearMarkers(t)
			t.Setenv(a.EnvVars[0], "1")
			got, ok := FromEnv()
			require.True(t, ok)
			assert.Equal(t, a.ID, got.ID)
		})
	}

	t.Run("marker set to 0 does not count", func(t *testing.T) {
		// An explicit MARKER=0 is someone saying "not under it" -- reading it
		// as the agent would make the opt-out set the very thing it denies.
		clearMarkers(t)
		t.Setenv("CLAUDECODE", "0")
		_, ok := FromEnv()
		assert.False(t, ok)
	})

	t.Run("empty marker does not count", func(t *testing.T) {
		clearMarkers(t)
		t.Setenv("CLAUDECODE", "")
		_, ok := FromEnv()
		assert.False(t, ok)
	})
}

func TestForProcess(t *testing.T) {
	cases := map[string]string{
		"claude":          "claude",
		"claude-code":     "claude", // prefix match: comm is truncated to 15 bytes
		"grok":            "grok",
		"xai-grok-pager":  "grok",
		"codex":           "codex",
		"gemini":          "gemini",
		"opencode":        "opencode",
		"opencode-tui":    "opencode",
		"bash":            "",
		"":                "",
		"notclaude":       "", // prefix, not substring
		"my-gemini-thing": "",
	}
	for comm, wantID := range cases {
		t.Run(comm, func(t *testing.T) {
			a, ok := ForProcess(comm)
			if wantID == "" {
				assert.False(t, ok, "comm %q must not match %q", comm, a.ID)
				return
			}
			require.True(t, ok, "comm %q should match %q", comm, wantID)
			assert.Equal(t, wantID, a.ID)
		})
	}
}

func TestIsPID(t *testing.T) {
	clearMarkers(t)
	assert.False(t, IsPID(1234))

	t.Setenv("OPENCODE_PID", "1234")
	assert.True(t, IsPID(1234))
	assert.False(t, IsPID(1235))

	a, ok := AgentForPID(1234)
	require.True(t, ok)
	assert.Equal(t, "opencode", a.ID)

	t.Setenv("OPENCODE_PID", "not-a-number")
	assert.False(t, IsPID(1234))
}

func TestDetectPrefersAncestryOverEnv(t *testing.T) {
	// Both signals present and disagreeing: ancestry wins, because a marker
	// is inherited by every descendant forever while an ancestor process IS
	// the lineage.
	clearMarkers(t)
	t.Setenv("CLAUDECODE", "1")

	chain := fakeChain{2: {"grok", 1}}
	got, ok := AncestorAgent(2, chain.lookup)
	require.True(t, ok)
	assert.Equal(t, "grok", got.ID, "ancestry must not be overridden by an inherited marker")

	// And with no ancestry to find, the marker is what answers.
	fromEnv, ok := FromEnv()
	require.True(t, ok)
	assert.Equal(t, "claude", fromEnv.ID)
}

func TestIsUsesDetect(t *testing.T) {
	clearMarkers(t)
	if _, underAgent := ProcessAncestor(); underAgent {
		t.Skip("this test process is itself under an agent; ancestry would answer instead of the marker")
	}
	t.Setenv("GEMINI_CLI", "1")
	assert.True(t, Is("gemini"))
	assert.False(t, Is("claude"))
	assert.False(t, Is("no-such-agent"))
}

func TestIsCapturePath(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "abc-123")
	assert.True(t, IsCapturePath("/tmp/claude-0/-home-user/abc-123/tasks/x.output"))
	assert.True(t, IsCapturePath("/somewhere/else/abc-123/whatever"))

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	// Structural fallback: ".output" under a claude-ish path.
	assert.True(t, IsCapturePath("/tmp/claude-0/tasks/task.output"))
	assert.True(t, IsCapturePath("/TMP/CLAUDE/TASKS/TASK.OUTPUT"), "matching is case-insensitive")

	// Ordinary redirects an agent might introduce stay unrecognized -- that
	// is the whole point of the function.
	assert.False(t, IsCapturePath("/dev/null"))
	assert.False(t, IsCapturePath("/tmp/out.log"))
	assert.False(t, IsCapturePath("/tmp/claude/out.log"))
	assert.False(t, IsCapturePath("/tmp/build.output"))
}

func TestDetectDoesNotPanicOnThisMachine(t *testing.T) {
	// Whatever this machine's process tree looks like, the real entry points
	// must answer without blowing up -- including inside a PID namespace
	// where the chain ends immediately.
	a, ok := Detect()
	if ok {
		assert.NotEmpty(t, a.ID)
		assert.NotEmpty(t, a.Name)
	}
	_ = IsAncestorPID(os.Getpid())
	_ = IsPipeReader("bash", os.Getppid())
}
