// Package notifysend sends Linux desktop notifications via the notify-send
// binary (part of libnotify).
package notifysend

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/felipeelias/claude-notifier/internal/notifier"
	"github.com/felipeelias/claude-notifier/internal/tmpl"
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

// mapUrgency resolves the configured urgency into a concrete notify-send value.
// Empty or "auto" → derive from NotificationType; anything else → pass through
// so users can opt into future notify-send values without a plugin update.
func mapUrgency(configured, notifType string) string {
	switch configured {
	case "", "auto":
		if notifType == "permission_prompt" {
			return "critical"
		}

		return "normal"
	default:
		return configured
	}
}

func (n *NotifySend) Send(ctx context.Context, notif notifier.Notification) error {
	tctx := tmpl.BuildContext(notif, n.Vars)

	msgTmpl := n.Message
	if msgTmpl == "" {
		msgTmpl = "{{.Message}}"
	}
	body, err := tmpl.Render("message", msgTmpl, tctx)
	if err != nil {
		return err
	}

	titleTmpl := n.Title
	if titleTmpl == "" {
		titleTmpl = "Claude Code ({{.Project}})"
	}
	title, err := tmpl.Render("title", titleTmpl, tctx)
	if err != nil {
		return err
	}

	args := []string{"-u", mapUrgency(n.Urgency, notif.NotificationType)}
	if n.AppName != "" {
		args = append(args, "-a", n.AppName)
	}
	if n.Icon != "" {
		args = append(args, "-i", n.Icon)
	}
	if n.ExpireTime != 0 {
		args = append(args, "-t", strconv.Itoa(n.ExpireTime))
	}
	args = append(args, title, body)

	cmd := exec.CommandContext(ctx, n.Path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("running %s: %s: %w", n.Path, string(output), err)
	}

	return nil
}
