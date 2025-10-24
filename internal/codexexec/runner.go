package codexexec

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

const (
	internalOriginatorEnv = "CODEX_INTERNAL_ORIGINATOR_OVERRIDE"
	goSDKOriginator       = "codex_sdk_go"
)

// Args mirrors the CLI flags accepted by `codex exec`.
type Args struct {
	Input            string
	BaseURL          string
	APIKey           string
	ThreadID         string
	Model            string
	SandboxMode      string
	WorkingDirectory string
	SkipGitRepoCheck bool
	OutputSchemaPath string
	Images           []string
}

// Runner wraps execution of the Codex CLI.
type Runner struct {
	executablePath string
}

// New constructs a Runner, optionally overriding the codex binary path.
func New(override string) (*Runner, error) {
	path := override
	if path == "" {
		var err error
		path, err = findCodexPath()
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("unable to locate codex binary at %q: %w", path, err)
	}
	return &Runner{executablePath: path}, nil
}

// Run executes `codex exec --experimental-json` and streams each JSONL line through handleLine.
func (r *Runner) Run(ctx context.Context, args Args, handleLine func([]byte) error) error {
	commandArgs := []string{"exec", "--experimental-json"}

	if args.Model != "" {
		commandArgs = append(commandArgs, "--model", args.Model)
	}
	if args.SandboxMode != "" {
		commandArgs = append(commandArgs, "--sandbox", args.SandboxMode)
	}
	if args.WorkingDirectory != "" {
		commandArgs = append(commandArgs, "--cd", args.WorkingDirectory)
	}
	if args.SkipGitRepoCheck {
		commandArgs = append(commandArgs, "--skip-git-repo-check")
	}
	if args.OutputSchemaPath != "" {
		commandArgs = append(commandArgs, "--output-schema", args.OutputSchemaPath)
	}
	for _, image := range args.Images {
		if image != "" {
			commandArgs = append(commandArgs, "--image", image)
		}
	}
	if args.ThreadID != "" {
		commandArgs = append(commandArgs, "resume", args.ThreadID)
	}

	cmd := exec.CommandContext(ctx, r.executablePath, commandArgs...)
	cmd.Env = buildEnv(args.BaseURL, args.APIKey)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("opening stdin: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("opening stdout: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("opening stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting codex exec: %w", err)
	}

	if _, err := io.WriteString(stdin, args.Input); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return fmt.Errorf("writing prompt to codex stdin: %w", err)
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("closing codex stdin: %w", err)
	}

	var stderrBuf bytes.Buffer
	var stderrWG sync.WaitGroup
	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	scanner := bufio.NewScanner(stdout)
	const maxLineSize = 4 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxLineSize)

	readErr := func() error {
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...) // copy to avoid reuse
			if err := handleLine(line); err != nil {
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				return err
			}
		}
		return scanner.Err()
	}()

	waitErr := cmd.Wait()
	stderrWG.Wait()

	if readErr != nil {
		return fmt.Errorf("reading codex output: %w", readErr)
	}

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			return fmt.Errorf("codex exec failed with code %d: %s", exitErr.ExitCode(), stderrBuf.String())
		}
		return fmt.Errorf("codex exec failed: %w", waitErr)
	}

	return nil
}

func buildEnv(baseURL, apiKey string) []string {
	envMap := make(map[string]string)
	for _, kv := range os.Environ() {
		if i := indexByte(kv, '='); i >= 0 {
			envMap[kv[:i]] = kv[i+1:]
		}
	}
	if _, ok := envMap[internalOriginatorEnv]; !ok {
		envMap[internalOriginatorEnv] = goSDKOriginator
	}
	if baseURL != "" {
		envMap["OPENAI_BASE_URL"] = baseURL
	}
	if apiKey != "" {
		envMap["CODEX_API_KEY"] = apiKey
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	return env
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func findCodexPath() (string, error) {
	path, err := exec.LookPath("codex")
	if err != nil {
		return "", fmt.Errorf("codex binary not found in PATH: %w", err)
	}
	return path, nil
}
