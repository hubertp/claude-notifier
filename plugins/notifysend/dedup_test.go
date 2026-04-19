package notifysend

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
