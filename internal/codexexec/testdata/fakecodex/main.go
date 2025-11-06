package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func main() {
	pidFile := os.Getenv("CODEX_FAKE_PID_FILE")
	if pidFile == "" {
		fmt.Fprintln(os.Stderr, "CODEX_FAKE_PID_FILE not set")
		os.Exit(2)
	}

	// Drain stdin to avoid the parent process blocking while sending a prompt.
	go io.Copy(io.Discard, os.Stdin)

	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write pid file: %v\n", err)
		os.Exit(3)
	}

	// Block until a termination signal arrives. If the parent issues SIGKILL the
	// process will exit immediately without delivering a signal on sigCh, which
	// is fine for the integration test.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}
