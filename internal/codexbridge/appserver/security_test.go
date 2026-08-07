package appserver

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClientWithoutTransportOrLauncherIsUnavailable(t *testing.T) {
	client := NewClient(ClientOptions{})
	if err := client.initialize(context.Background(), "/repo"); Code(err) != "HOST_BRIDGE_UNAVAILABLE" {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexLauncherUsesExactArgumentsAndExplicitEnvironment(t *testing.T) {
	directory := t.TempDir()
	capture := filepath.Join(directory, "capture.txt")
	script := filepath.Join(directory, "fake-codex")
	content := "#!/bin/sh\n" +
		"{ printf 'args\\n'; printf '%s\\n' \"$@\"; printf 'env\\n'; env | sort; } > \"$CAPTURE_PATH\"\n" +
		"while IFS= read -r line; do :; done\n"
	if err := os.WriteFile(script, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	environment := []string{"CAPTURE_PATH=" + capture, "PATH=" + os.Getenv("PATH"), "OAW_TEST_VALUE=present"}
	transport, err := (CodexLauncher{Binary: script, Environment: environment}).Open(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = transport.Close() }()

	deadline := time.Now().Add(2 * time.Second)
	var raw []byte
	for time.Now().Before(deadline) {
		raw, err = os.ReadFile(capture)
		if err == nil && bytes.Contains(raw, []byte("OAW_TEST_VALUE=present")) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || !bytes.Contains(raw, []byte("OAW_TEST_VALUE=present")) {
		t.Fatalf("capture did not complete: %v: %q", err, raw)
	}
	text := string(raw)
	if !strings.Contains(text, "args\napp-server\n--listen\nstdio://\n") ||
		!strings.Contains(text, "OAW_TEST_VALUE=present") || strings.Contains(text, "CODEX_HOME=") || strings.Contains(text, "HOME=") ||
		strings.Contains(text, "thread/start") || strings.Contains(text, "turn/start") || strings.Contains(text, "dangerously-bypass") {
		t.Fatalf("launcher capture:\n%s", text)
	}
}

func TestJSONLineFramingIsBounded(t *testing.T) {
	var output bytes.Buffer
	if err := writeJSONLine(&output, []byte(`{"ok":true}`), 64); err != nil {
		t.Fatal(err)
	}
	if output.String() != "{\"ok\":true}\n" {
		t.Fatalf("line = %q", output.String())
	}
	if err := writeJSONLine(&output, bytes.Repeat([]byte{'x'}, 64), 64); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("oversized write error = %v", err)
	}
	if _, err := readJSONLine(bufio.NewReader(strings.NewReader(strings.Repeat("x", 65)+"\n")), 64); Code(err) != "HOST_OBSERVATION_FAILED" {
		t.Fatalf("oversized read error = %v", err)
	}
}

func TestCodexLauncherDoesNotFallBackFromMissingBinary(t *testing.T) {
	_, err := (CodexLauncher{Binary: filepath.Join(t.TempDir(), "missing-codex")}).Open(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("missing Codex binary unexpectedly opened a transport")
	}
}
