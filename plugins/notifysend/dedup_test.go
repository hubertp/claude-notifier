package notifysend

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
