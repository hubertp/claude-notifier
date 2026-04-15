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

func TestUrgencyAutoMap(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		notifType  string
		want       string
	}{
		{"auto with permission_prompt", "auto", "permission_prompt", "critical"},
		{"auto with idle_prompt", "auto", "idle_prompt", "normal"},
		{"auto with auth_success", "auto", "auth_success", "normal"},
		{"auto with elicitation_dialog", "auto", "elicitation_dialog", "normal"},
		{"auto with unknown type", "auto", "something_else", "normal"},
		{"auto with empty type", "auto", "", "normal"},
		{"empty config with permission_prompt", "", "permission_prompt", "critical"},
		{"empty config with idle_prompt", "", "idle_prompt", "normal"},
		{"explicit low", "low", "permission_prompt", "low"},
		{"explicit normal", "normal", "permission_prompt", "normal"},
		{"explicit critical", "critical", "idle_prompt", "critical"},
		{"passthrough unknown", "panic", "idle_prompt", "panic"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin, logFile := fakeBinary(t)
			p := &ns.NotifySend{
				Path:    bin,
				Message: "{{.Message}}",
				Title:   "t",
				Urgency: tc.configured,
			}
			err := p.Send(context.Background(), notifier.Notification{
				Message:          "m",
				NotificationType: tc.notifType,
			})
			require.NoError(t, err)

			args := readArgs(t, logFile)
			assertArgPair(t, args, "-u", tc.want)
		})
	}
}

func TestSendTemplateRendering(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:    bin,
		Message: "**{{.Project}}**: {{.Message}}",
		Title:   "{{.NotificationType}}: {{.Title}}",
		Urgency: "normal",
	}
	err := p.Send(context.Background(), notifier.Notification{
		Message:          "Build complete",
		Title:            "Claude Code",
		Cwd:              "/home/user/myproject",
		NotificationType: "idle_prompt",
	})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "idle_prompt: Claude Code", args[len(args)-2])
	assert.Equal(t, "**myproject**: Build complete", args[len(args)-1])
}

func TestSendWithVars(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:    bin,
		Message: "{{.Env}}: {{.Message}}",
		Title:   "{{.Title}}",
		Urgency: "normal",
		Vars:    map[string]string{"env": "production"},
	}
	err := p.Send(context.Background(), notifier.Notification{
		Message: "done",
		Title:   "test",
	})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	require.GreaterOrEqual(t, len(args), 1)
	assert.Equal(t, "production: done", args[len(args)-1])
}

func TestSendBadTemplate(t *testing.T) {
	bin, _ := fakeBinary(t)

	p := &ns.NotifySend{
		Path:    bin,
		Message: "{{.Invalid",
		Title:   "t",
		Urgency: "normal",
	}
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rendering message template")
}

func TestSendBinaryNotFound(t *testing.T) {
	p := &ns.NotifySend{
		Path:    "/nonexistent/notify-send",
		Message: "{{.Message}}",
		Title:   "t",
		Urgency: "normal",
	}
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running /nonexistent/notify-send")
}

func TestSendBinaryNonZeroExit(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "notify-send")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho 'error' >&2\nexit 1\n"), 0755))

	p := &ns.NotifySend{
		Path:    script,
		Message: "{{.Message}}",
		Title:   "t",
		Urgency: "normal",
	}
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running")
}

func TestSendAllFlags(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:       bin,
		Message:    "{{.Message}}",
		Title:      "{{.Title}}",
		AppName:    "My App",
		Urgency:    "critical",
		Icon:       "dialog-warning",
		ExpireTime: 5000,
	}
	err := p.Send(context.Background(), notifier.Notification{
		Message: "m",
		Title:   "t",
	})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	assertArgPair(t, args, "-u", "critical")
	assertArgPair(t, args, "-a", "My App")
	assertArgPair(t, args, "-i", "dialog-warning")
	assertArgPair(t, args, "-t", "5000")

	// Positional order preserved even with all optional flags present.
	require.GreaterOrEqual(t, len(args), 2)
	assert.Equal(t, "t", args[len(args)-2])
	assert.Equal(t, "m", args[len(args)-1])
}

func TestSendExpireTimeZeroOmitted(t *testing.T) {
	bin, logFile := fakeBinary(t)

	p := &ns.NotifySend{
		Path:       bin,
		Message:    "{{.Message}}",
		Title:      "{{.Title}}",
		Urgency:    "normal",
		ExpireTime: 0,
	}
	err := p.Send(context.Background(), notifier.Notification{
		Message: "m",
		Title:   "t",
	})
	require.NoError(t, err)

	args := readArgs(t, logFile)
	assert.NotContains(t, args, "-t", "expire_time=0 must not emit -t (DE default)")
	assert.NotContains(t, args, "-i", "icon unset must not emit -i")
}
