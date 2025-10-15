package godex

import "github.com/activadee/godex/internal/codexexec"

// Codex is the entrypoint for interacting with the Codex agent via the CLI.
type Codex struct {
	exec    *codexexec.Runner
	options CodexOptions
}

// New constructs a Codex SDK instance. The Codex binary is discovered automatically unless
// CodexOptions.CodexPathOverride is provided.
func New(options CodexOptions) (*Codex, error) {
	exec, err := codexexec.New(options.CodexPathOverride)
	if err != nil {
		return nil, err
	}
	return &Codex{
		exec:    exec,
		options: options,
	}, nil
}

// StartThread opens a new thread with the agent.
func (c *Codex) StartThread(options ThreadOptions) *Thread {
	return newThread(c.exec, c.options, options, "")
}

// ResumeThread recreates a thread using a previously obtained thread identifier.
func (c *Codex) ResumeThread(id string, options ThreadOptions) *Thread {
	return newThread(c.exec, c.options, options, id)
}
