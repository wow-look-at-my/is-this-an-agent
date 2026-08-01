# is-this-an-agent

Is this process running under an AI coding agent, and which one? A Go package
and a set of standalone shell scripts that answer that, from one shared roster
of what each agent looks like.

Known agents: **Claude Code**, **grok build**, **Codex**, **Gemini CLI**,
**opencode**.

## Shell

Each script is self-contained — copy one anywhere and run it. Exit status 0
means yes, 1 means no; `-q` prints nothing.

```sh
scripts/is-this-an-agent.sh        # prints the agent's name, e.g. "Claude"
scripts/is-this-claude.sh -q && echo "under Claude"
```

One script per agent: `is-this-{an-agent,claude,grok,codex,gemini,opencode}.sh`.

## Go

```go
import agent "github.com/wow-look-at-my/is-this-an-agent"

if a, ok := agent.Detect(); ok {
        fmt.Printf("running under %s\n", a.Name)
}
```

`Detect` checks process ancestry first and environment markers second. Also
available: `Is("claude")`, `Roster()`, `FromEnv()`, `ProcessAncestor()`,
`ForProcess(comm)`, and `IsPipeReader(comm, pid)` for tools that need to know
whether the agent itself is reading their output.

## Detection is advisory

Any process can set `CLAUDECODE=1`, and an agent can be renamed out of the
roster. This tells a tool how to behave, and proves nothing to an adversary.

Adding an agent means adding it to the roster in `agent.go` **and** adding its
script; the tests fail on either half alone.
