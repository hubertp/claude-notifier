package notifysend_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/felipeelias/claude-notifier/internal/notifier"
	ns "github.com/felipeelias/claude-notifier/plugins/notifysend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	p := &ns.NotifySend{}
	assert.Equal(t, "notify-send", p.Name())
}

func TestImplementsNotifier(t *testing.T) {
	var _ notifier.Notifier = &ns.NotifySend{}
}

func TestDefaults(t *testing.T) {
	p := &ns.NotifySend{}
	ns.ApplyDefaults(p)
	assert.Equal(t, "notify-send", p.Path)
	assert.Equal(t, "{{.Message}}", p.Message)
	assert.Equal(t, "Claude Code ({{.Project}})", p.Title)
	assert.Equal(t, "Claude Code", p.AppName)
	assert.Equal(t, "auto", p.Urgency)
}

// fakeBinary creates a shell script that logs all args to a file, newline-separated.
// Returns the script path and the log file path.
func fakeBinary(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "args.log")
	script := filepath.Join(dir, "notify-send")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %s\n", logFile)
	require.NoError(t, os.WriteFile(script, []byte(content), 0755))

	return script, logFile
}

// readArgs reads the logged args from the fake binary.
func readArgs(t *testing.T, logFile string) []string {
	t.Helper()
	data, err := os.ReadFile(logFile)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// assertArgPair verifies that flag is immediately followed by value in args.
func assertArgPair(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, arg := range args {
		if arg == flag {
			require.Less(t, i+1, len(args), "flag %s has no value", flag)
			assert.Equal(t, value, args[i+1], "flag %s value mismatch", flag)

			return
		}
	}
	t.Errorf("flag %s not found in args", flag)
}

func TestSend(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:    bin,
		Message: "{{.Message}}",
		Title:   "{{.Title}}",
		AppName: "Claude Code",
		Urgency: "auto",
	}
	err := p.Send(context.Background(), notifier.Notification{
		Message: "Task complete",
		Title:   "Claude Code",
	})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	assertArgPair(t, args, "-u", "normal")
	assertArgPair(t, args, "-a", "Claude Code")

	// Positional args: title, then message, must be the last two entries.
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "Claude Code", args[len(args)-2])
	assert.Equal(t, "Task complete", args[len(args)-1])
}

func TestPositionalOrder(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:    bin,
		Message: "body-text",
		Title:   "title-text",
		Urgency: "low",
	}
	err := p.Send(context.Background(), notifier.Notification{Message: "unused"})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "title-text", args[len(args)-2], "title must be second-to-last")
	assert.Equal(t, "body-text", args[len(args)-1], "body must be last")
}
