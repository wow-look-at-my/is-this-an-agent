package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChain is a process tree: pid -> (comm, ppid). It is what makes the walk
// testable at all -- the real one reads /proc, and no test can arrange for its
// own machine to have an ancestor named `codex`.
type fakeChain map[int]struct {
	comm string
	ppid int
}

func (c fakeChain) lookup(pid int) (string, int, bool) {
	e, ok := c[pid]
	if !ok {
		return "", 0, false
	}
	return e.comm, e.ppid, true
}

func TestAncestorAgent(t *testing.T) {
	t.Run("finds an agent several hops up", func(t *testing.T) {
		clearMarkers(t)
		chain := fakeChain{
			10: {"bash", 9},
			9:  {"sh", 8},
			8:  {"claude", 1},
		}
		a, ok := AncestorAgent(10, chain.lookup)
		require.True(t, ok)
		assert.Equal(t, "claude", a.ID)
	})

	t.Run("no agent in the chain", func(t *testing.T) {
		clearMarkers(t)
		chain := fakeChain{10: {"bash", 9}, 9: {"tmux", 1}}
		_, ok := AncestorAgent(10, chain.lookup)
		assert.False(t, ok)
	})

	t.Run("stops at an unreadable process", func(t *testing.T) {
		clearMarkers(t)
		// A chain that dead-ends must end the walk, not loop or panic.
		chain := fakeChain{10: {"bash", 9}}
		_, ok := AncestorAgent(10, chain.lookup)
		assert.False(t, ok)
	})

	t.Run("a cycle terminates", func(t *testing.T) {
		clearMarkers(t)
		// pid-reuse can produce a chain that never reaches PID 1. The hop
		// bound is what stops this from hanging the caller forever.
		chain := fakeChain{10: {"bash", 11}, 11: {"sh", 10}}
		_, ok := AncestorAgent(10, chain.lookup)
		assert.False(t, ok)
	})

	t.Run("finds an agent by its PID variable", func(t *testing.T) {
		// opencode running from a JS runtime: the process name is the
		// runtime's, so the only handle is the PID it exported.
		clearMarkers(t)
		t.Setenv("OPENCODE_PID", "9")
		chain := fakeChain{
			10: {"bash", 9},
			9:  {"bun", 1},
		}
		a, ok := AncestorAgent(10, chain.lookup)
		require.True(t, ok)
		assert.Equal(t, "opencode", a.ID)
	})

	t.Run("start at PID 1 walks nothing", func(t *testing.T) {
		clearMarkers(t)
		chain := fakeChain{1: {"claude", 0}}
		_, ok := AncestorAgent(1, chain.lookup)
		assert.False(t, ok, "PID 1 is the stop condition, not a process to inspect")
	})
}

func TestAncestorPID(t *testing.T) {
	chain := fakeChain{
		10: {"bash", 9},
		9:  {"sh", 8},
		8:  {"claude", 1},
	}
	assert.True(t, AncestorPID(10, 10, chain.lookup), "the start pid counts as its own ancestor")
	assert.True(t, AncestorPID(10, 8, chain.lookup))
	assert.False(t, AncestorPID(10, 7, chain.lookup))

	// A truncated chain cannot prove ancestry, so it must not claim it.
	assert.False(t, AncestorPID(10, 99, fakeChain{10: {"bash", 9}}.lookup))
}

func TestPipeReader(t *testing.T) {
	chain := fakeChain{
		10: {"bash", 9},
		9:  {"claude", 1},
	}

	t.Run("the agent reading our pipe is allowed", func(t *testing.T) {
		clearMarkers(t)
		assert.True(t, PipeReader("claude", 9, 10, chain.lookup))
	})

	t.Run("a pipeline filter is not", func(t *testing.T) {
		// `go-toolchain | head`: head is a sibling, not an ancestor, and the
		// output it swallows is output the agent never sees.
		clearMarkers(t)
		assert.False(t, PipeReader("head", 42, 10, chain.lookup))
	})

	t.Run("the shell of a $(...) capture is not", func(t *testing.T) {
		clearMarkers(t)
		assert.False(t, PipeReader("bash", 10, 10, chain.lookup),
			"an ancestor that is not an agent must not grant the allowance")
	})

	t.Run("an agent that named its own PID is allowed", func(t *testing.T) {
		clearMarkers(t)
		t.Setenv("OPENCODE_PID", "9")
		assert.True(t, PipeReader("bun", 9, 10, chain.lookup))
	})
}
