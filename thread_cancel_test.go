package godex

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/activadee/godex/internal/codexexec"
)

func TestThreadRunStreamedCancellationTerminatesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cancellation integration test relies on unix signals")
	}

	fakeBinary := buildFakeCodexBinary(t)

	runner, err := codexexec.New(codexexec.RunnerOptions{PathOverride: fakeBinary})
	if err != nil {
		t.Fatalf("codexexec.New returned error: %v", err)
	}

	pidFile := filepath.Join(t.TempDir(), "fake-codex.pid")
	t.Setenv("CODEX_FAKE_PID_FILE", pidFile)

	thread := newThread(runner, CodexOptions{}, ThreadOptions{}, "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result, err := thread.RunStreamed(ctx, "long running turn", nil)
	if err != nil {
		t.Fatalf("RunStreamed returned error: %v", err)
	}
	defer result.Close()

	eventsDrained := make(chan struct{})
	go func() {
		for range result.Events() {
			// drain events until stream closes
		}
		close(eventsDrained)
	}()
	defer func() { <-eventsDrained }()

	pid := waitForPIDFile(t, pidFile)

	cancel()

	if err := result.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("result.Wait error = %v, want context.Canceled", err)
	}

	waitForProcessExit(t, pid)
}

func buildFakeCodexBinary(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	binaryPath := filepath.Join(binDir, "codex")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./internal/codexexec/testdata/fakecodex")
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake codex binary: %v\n%s", err, output)
	}

	return binaryPath
}

func waitForPIDFile(t *testing.T, pidFile string) int {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pidStr := strings.TrimSpace(string(data))
			pid, convErr := strconv.Atoi(pidStr)
			if convErr != nil {
				t.Fatalf("unexpected pid file contents %q: %v", pidStr, convErr)
			}
			if pid <= 0 {
				t.Fatalf("invalid pid %d", pid)
			}
			return pid
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("reading pid file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for pid file %s", pidFile)
	return 0
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil {
			t.Fatalf("checking process %d status: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("process %d still running after cancellation", pid)
}
