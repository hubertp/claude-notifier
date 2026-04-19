package notifysend

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/felipeelias/claude-notifier/internal/notifier"
	"github.com/stretchr/testify/assert"
)

func TestStateDirXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdg)

	got, err := stateDir()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(xdg, "claude-notifier", "notify-send"), got)

	info, err := os.Stat(got)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestStateDirFallback(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	got, err := stateDir()
	assert.NoError(t, err)

	expected := filepath.Join(tmp, "claude-notifier-"+strconv.Itoa(os.Getuid()), "notify-send")
	assert.Equal(t, expected, got)

	info, err := os.Stat(got)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestDedupFilename(t *testing.T) {
	// Deterministic: same input → same output.
	a := dedupFilename("abc123")
	b := dedupFilename("abc123")
	assert.Equal(t, a, b)

	// 16 hex chars + ".id" = 19 chars.
	assert.Len(t, a, 19)
	assert.True(t, strings.HasSuffix(a, ".id"))

	// Different keys produce different filenames.
	assert.NotEqual(t, dedupFilename("abc"), dedupFilename("xyz"))

	// Filesystem-safe: only hex + ".id".
	name := strings.TrimSuffix(a, ".id")
	for _, c := range name {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"non-hex char in filename: %q", c)
	}
}

func TestReadIDMissing(t *testing.T) {
	got := readID(filepath.Join(t.TempDir(), "nope.id"))
	assert.Equal(t, 0, got)
}

func TestReadIDParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id")
	assert.NoError(t, os.WriteFile(path, []byte("42"), 0o600))

	assert.Equal(t, 42, readID(path))
}

func TestReadIDTrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id")
	assert.NoError(t, os.WriteFile(path, []byte("  99\n"), 0o600))

	assert.Equal(t, 99, readID(path))
}

func TestReadIDCorrupt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id")
	assert.NoError(t, os.WriteFile(path, []byte("garbage"), 0o600))

	assert.Equal(t, 0, readID(path), "unparseable contents must be treated as missing")
}

func TestWriteIDAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id")

	err := writeID(path, 1234)
	assert.NoError(t, err)

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "1234", string(data))

	// No orphan temp files left behind.
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "id", entries[0].Name())
}

func TestWriteIDOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "id")

	assert.NoError(t, writeID(path, 1))
	assert.NoError(t, writeID(path, 2))

	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "2", string(data))
}

// newNotifier returns a NotifySend wired up for dedup tests: fake binary
// captured via the standard pattern, ReplaceKey preset. The caller sets
// FAKE_NOTIFY_SEND_ID via t.Setenv before Send to control the fake's stdout.
func newNotifier(t *testing.T, replaceKey string) (*NotifySend, string) {
	t.Helper()
	dir := t.TempDir()
	logFile := filepath.Join(dir, "args.log")
	script := filepath.Join(dir, "notify-send")
	content := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + logFile + "\n" +
		"if [ -n \"$FAKE_NOTIFY_SEND_ID\" ]; then printf '%s' \"$FAKE_NOTIFY_SEND_ID\"; fi\n"
	assert.NoError(t, os.WriteFile(script, []byte(content), 0o755))

	return &NotifySend{
		Path:       script,
		Message:    "{{.Message}}",
		Title:      "t",
		Urgency:    "normal",
		ReplaceKey: replaceKey,
	}, logFile
}

func readArgsInternal(t *testing.T, logFile string) []string {
	t.Helper()
	data, err := os.ReadFile(logFile)
	assert.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func containsFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func flagValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func TestDedupFirstSend(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")

	p, logFile := newNotifier(t, "session-abc")
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	args := readArgsInternal(t, logFile)
	assert.True(t, containsFlag(args, "-p"), "first send must include -p")
	assert.False(t, containsFlag(args, "-r"), "first send must not include -r")

	dir, err := stateDir()
	assert.NoError(t, err)
	assert.Equal(t, 42, readID(filepath.Join(dir, dedupFilename("session-abc"))))
}

func TestDedupReplaceSend(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	dir, err := stateDir()
	assert.NoError(t, err)
	assert.NoError(t, writeID(filepath.Join(dir, dedupFilename("session-abc")), 42))

	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")
	p, logFile := newNotifier(t, "session-abc")
	err = p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	args := readArgsInternal(t, logFile)
	rVal, ok := flagValue(args, "-r")
	assert.True(t, ok, "replace send must include -r")
	assert.Equal(t, "42", rVal)
	assert.True(t, containsFlag(args, "-p"), "replace send must still include -p")
}

func TestDedupNewIDOverwrites(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	dir, err := stateDir()
	assert.NoError(t, err)
	statePath := filepath.Join(dir, dedupFilename("session-abc"))
	assert.NoError(t, writeID(statePath, 42))

	t.Setenv("FAKE_NOTIFY_SEND_ID", "99")
	p, _ := newNotifier(t, "session-abc")
	err = p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	assert.Equal(t, 99, readID(statePath))
}

func TestDedupEmptyReplaceKey(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")

	p, logFile := newNotifier(t, "")
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	args := readArgsInternal(t, logFile)
	assert.False(t, containsFlag(args, "-r"))
	assert.False(t, containsFlag(args, "-p"))

	dir, err := stateDir()
	assert.NoError(t, err)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)
	assert.Empty(t, entries, "empty replace_key must not create any state files")
}

func TestDedupReplaceKeyTemplate(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")

	p, _ := newNotifier(t, "{{.SessionID}}")

	err := p.Send(context.Background(), notifier.Notification{
		Message:   "hi",
		SessionID: "alpha",
	})
	assert.NoError(t, err)

	t.Setenv("FAKE_NOTIFY_SEND_ID", "77")
	err = p.Send(context.Background(), notifier.Notification{
		Message:   "hi",
		SessionID: "beta",
	})
	assert.NoError(t, err)

	dir, err := stateDir()
	assert.NoError(t, err)
	assert.Equal(t, 42, readID(filepath.Join(dir, dedupFilename("alpha"))))
	assert.Equal(t, 77, readID(filepath.Join(dir, dedupFilename("beta"))))
}

func TestDedupBadTemplate(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	p, _ := newNotifier(t, "{{.Invalid")
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "replace_key")
}

func TestDedupUnparseableStdout(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("FAKE_NOTIFY_SEND_ID", "not-a-number")

	p, _ := newNotifier(t, "session-abc")
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err, "unparseable output is not a send-level error")

	dir, err := stateDir()
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, dedupFilename("session-abc")))
	assert.True(t, os.IsNotExist(err), "no state file written when ID unparseable")
}

func TestDedupCorruptStateFile(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	dir, err := stateDir()
	assert.NoError(t, err)
	statePath := filepath.Join(dir, dedupFilename("session-abc"))
	assert.NoError(t, os.WriteFile(statePath, []byte("garbage"), 0o600))

	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")
	p, logFile := newNotifier(t, "session-abc")
	err = p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	args := readArgsInternal(t, logFile)
	assert.False(t, containsFlag(args, "-r"), "corrupt state must not emit -r")

	assert.Equal(t, 42, readID(statePath), "corrupt state is overwritten on success")
}

func TestDedupAtomicWrite(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("FAKE_NOTIFY_SEND_ID", "42")

	p, _ := newNotifier(t, "session-abc")
	err := p.Send(context.Background(), notifier.Notification{Message: "hi"})
	assert.NoError(t, err)

	dir, err := stateDir()
	assert.NoError(t, err)
	entries, err := os.ReadDir(dir)
	assert.NoError(t, err)

	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"),
			"no orphan .tmp files after successful send: %s", e.Name())
	}
}
