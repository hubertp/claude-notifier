package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testBinary string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "claude-notifier-test-*")
	if err != nil {
		log.Fatal(err)
	}

	testBinary = filepath.Join(tmp, "claude-notifier")
	build := exec.CommandContext(context.Background(), "go", "build", "-o", testBinary, ".")
	build.Stderr = os.Stderr

	err = build.Run()
	if err != nil {
		_ = os.RemoveAll(tmp)
		log.Fatalf("failed to build binary: %v", err)
	}

	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	var gotBody string
	var gotTitle string
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotTitle = r.Header.Get("Title")
		gotHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	configContent := `[[notifiers.ntfy]]
url = "` + srv.URL + `"
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	input, err := json.Marshal(map[string]string{
		"message":           "Build complete",
		"title":             "Claude Code (myproject)",
		"cwd":               "/home/user/myproject",
		"notification_type": "idle_prompt",
		"session_id":        "abc123",
	})
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), testBinary, "--config", configPath)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	require.NoError(t, err, "stderr: %s", stderr.String())

	assert.Equal(t, "Build complete", gotBody)
	assert.Equal(t, "Claude Code (myproject)", gotTitle)
	assert.Equal(t, "yes", gotHeaders.Get("X-Markdown"))
}

func TestEndToEndMissingConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Run with nonexistent config — should still exit 0 (never fail the hook)
	input, err := json.Marshal(map[string]string{"message": "test"})
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), testBinary, "--config", "/nonexistent/config.toml")
	cmd.Stdin = bytes.NewReader(input)
	err = cmd.Run()
	assert.NoError(t, err, "should exit 0 even with missing config")
}

func TestEndToEndInitCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	configPath := filepath.Join(t.TempDir(), "claude-notifier", "config.toml")
	cmd := exec.CommandContext(context.Background(), testBinary, "--config", configPath, "init")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "Config created")

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "[[notifiers.notify-send]]")
	assert.Contains(t, string(content), "[[notifiers.ntfy]]")
	assert.Contains(t, string(content), "[[notifiers.terminal-notifier]]")
}

func TestEndToEndTestCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Start mock server
	var received bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	err := os.WriteFile(configPath, []byte(`[[notifiers.ntfy]]
url = "`+srv.URL+`"
`), 0644)
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), testBinary, "--config", configPath, "test")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = cmd.Run()
	require.NoError(t, err)

	assert.True(t, received, "mock server should have received the test notification")
	assert.Contains(t, stdout.String(), "Test notification sent successfully")
}

func TestEndToEndOversizedFieldsExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not have been contacted")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`[[notifiers.ntfy]]
url = "`+srv.URL+`"
`), 0644))

	input, err := json.Marshal(map[string]string{
		"message": strings.Repeat("x", 5000),
	})
	require.NoError(t, err)

	cmd := exec.CommandContext(context.Background(), testBinary, "--config", configPath)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	require.NoError(t, err, "should exit 0 even with oversized fields")
	assert.Contains(t, stderr.String(), "invalid notification")
}
