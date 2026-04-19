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
