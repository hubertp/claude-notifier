// Package notifysend sends Linux desktop notifications via the notify-send
// binary (part of libnotify).
package notifysend

import (
	"context"

	"github.com/felipeelias/claude-notifier/internal/notifier"
)

// NotifySend sends Linux desktop notifications via the notify-send binary.
type NotifySend struct {
	Path       string            `toml:"path"`
	Message    string            `toml:"message"`
	Title      string            `toml:"title"`
	AppName    string            `toml:"app_name"`
	Urgency    string            `toml:"urgency"`
	Icon       string            `toml:"icon"`
	ExpireTime int               `toml:"expire_time"`
	Vars       map[string]string `toml:"vars"`
}

// ApplyDefaults sets sane defaults on a new NotifySend instance.
func ApplyDefaults(n *NotifySend) {
	n.Path = "notify-send"
	n.Message = "{{.Message}}"
	n.Title = "Claude Code ({{.Project}})"
	n.AppName = "Claude Code"
	n.Urgency = "auto"
}

func (n *NotifySend) Name() string { return "notify-send" }

// Send is a placeholder until Task 2 implements it. It satisfies the Notifier
// interface so TestImplementsNotifier compiles; it will be replaced.
func (n *NotifySend) Send(ctx context.Context, notif notifier.Notification) error {
	return nil
}
