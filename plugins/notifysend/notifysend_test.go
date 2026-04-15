package notifysend_test

import (
	"testing"

	"github.com/felipeelias/claude-notifier/internal/notifier"
	ns "github.com/felipeelias/claude-notifier/plugins/notifysend"
	"github.com/stretchr/testify/assert"
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
