package godex

// ApprovalMode describes how the Codex CLI should request approval for actions that
// might require user consent. The Codex CLI itself interprets these values, the SDK
// merely forwards them when provided.
type ApprovalMode string

const (
	ApprovalModeNever     ApprovalMode = "never"
	ApprovalModeOnRequest ApprovalMode = "on-request"
	ApprovalModeOnFailure ApprovalMode = "on-failure"
	ApprovalModeUntrusted ApprovalMode = "untrusted"
)

// SandboxMode mirrors the CLI sandbox configuration that controls which filesystem
// operations the agent may perform.
type SandboxMode string

const (
	SandboxModeReadOnly         SandboxMode = "read-only"
	SandboxModeWorkspaceWrite   SandboxMode = "workspace-write"
	SandboxModeDangerFullAccess SandboxMode = "danger-full-access"
)

// CodexOptions configure the SDK itself rather than an individual thread.
type CodexOptions struct {
	// CodexPathOverride allows specifying the path to a Codex binary instead of the bundled one.
	CodexPathOverride string
	// BaseURL overrides the service endpoint used by the Codex CLI.
	BaseURL string
	// APIKey optionally overrides authentication for the Codex CLI. When empty, the CLI
	// falls back to its own configured credentials (e.g. environment variables or auth login).
	APIKey string
	// ConfigOverrides forwards CLI configuration overrides as `-c key=value` pairs. When
	// the `profile` key is present it is emitted as `--profile <value>` instead.
	ConfigOverrides map[string]any
	// CLICacheDir overrides the directory used to cache downloaded Codex binaries. When empty,
	// the SDK falls back to $GODEX_CLI_CACHE, then the user cache directory.
	CLICacheDir string
	// CLIReleaseTag pins the Codex CLI release tag to download. When unset, the SDK checks
	// $GODEX_CLI_RELEASE_TAG before falling back to its default bundled tag.
	CLIReleaseTag string
	// CLIChecksum optionally enforces integrity verification of the downloaded Codex binary.
	// Provide the expected SHA-256 checksum (hex encoded). When empty, checksum verification
	// is skipped. Use $GODEX_CLI_CHECKSUM to configure the same behavior via environment.
	CLIChecksum string
}

// ThreadOptions configure how the CLI executes a particular thread.
type ThreadOptions struct {
	// Model specifies the model identifier to use for the thread.
	Model string
	// SandboxMode controls the CLI sandbox setting (equivalent to `--sandbox` flag).
	SandboxMode SandboxMode
	// WorkingDirectory sets the working directory for the agent (`--cd` flag).
	WorkingDirectory string
	// SkipGitRepoCheck mirrors the CLI flag `--skip-git-repo-check`.
	SkipGitRepoCheck bool
}

// TurnOptions configure a single turn executed within a thread.
type TurnOptions struct {
	// OutputSchema is an optional JSON schema describing the structured response to
	// collect from the agent. Must serialize to a JSON object (not an array or primitive).
	OutputSchema any
	// Callbacks attaches optional streaming callbacks invoked as events arrive.
	Callbacks *StreamCallbacks
}
