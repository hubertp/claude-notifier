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
	ReplaceKey string            `toml:"replace_key"`
	Vars       map[string]string `toml:"vars"`
}

// ApplyDefaults sets sane defaults on a new NotifySend instance.
func ApplyDefaults(n *NotifySend) {
	n.Path = "notify-send"
	n.Message = "{{.Message}}"
	n.Title = "Claude Code ({{.Project}})"
	n.AppName = "Claude Code"
	n.Urgency = "auto"
	n.ReplaceKey = "{{.SessionID}}"
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

// SampleConfig returns example TOML configuration.
func (n *NotifySend) SampleConfig() string {
	return `## Linux desktop notifications via notify-send (libnotify)
## https://man.archlinux.org/man/notify-send.1
[[notifiers.notify-send]]

## Path to the notify-send binary (required)
## Install: apt install libnotify-bin  /  dnf install libnotify  /  pacman -S libnotify
path = "notify-send"

## Go template for the notification body
## Available variables: {{.Message}}, {{.Title}}, {{.Project}}, {{.Cwd}},
## {{.NotificationType}}, {{.SessionID}}, {{.TranscriptPath}}
## Custom variables from [notifiers.notify-send.vars] are also available, title-cased
# message = "{{.Message}}"

## Go template for the notification title
# title = "Claude Code ({{.Project}})"

## App name shown in the notification center
# app_name = "Claude Code"

## Urgency: "auto" (maps permission_prompt -> critical, else normal),
## or one of "low", "normal", "critical"
# urgency = "auto"

## Icon: a freedesktop icon name (e.g. "dialog-information",
## "dialog-warning", "emblem-important") or an absolute path to an image
# icon = ""

## Expiration in milliseconds. Omit (or leave at 0) to use the DE default.
## notify-send's own "0 = never" is shadowed: set a large value (e.g. 86400000)
## if you need effectively-never.
# expire_time = 0

## User-defined template variables
## Keys are title-cased for template access (env -> {{.Env}})
# [notifiers.notify-send.vars]
# env = "production"
`
}

// Register adds notify-send to the given plugin registry.
func Register(reg *notifier.Registry) {
	err := reg.Register("notify-send", func() notifier.Notifier {
		n := &NotifySend{}
		ApplyDefaults(n)

		return n
	})
	if err != nil {
		panic(err)
	}
}
