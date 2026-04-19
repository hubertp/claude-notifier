// Package notifysend sends Linux desktop notifications via the notify-send
// binary (part of libnotify).
package notifysend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

// dedupFilename returns the state filename for a rendered dedup key.
// sha256 → first 16 hex chars → ".id". Filesystem-safe and collision-
// resistant enough for per-user state (64-bit prefix).
func dedupFilename(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:8]) + ".id"
}

// stateDir returns the directory for per-key notification-ID state files,
// creating it on demand. Prefers $XDG_RUNTIME_DIR (tmpfs, cleared on
// logout/reboot — matches notify-send ID lifetime). Falls back to
// $TMPDIR/claude-notifier-<uid>/notify-send/ when XDG_RUNTIME_DIR is unset.
func stateDir() (string, error) {
	var root string
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		root = filepath.Join(xdg, "claude-notifier", "notify-send")
	} else {
		root = filepath.Join(os.TempDir(),
			fmt.Sprintf("claude-notifier-%d", os.Getuid()),
			"notify-send")
	}

	err := os.MkdirAll(root, 0o700)
	if err != nil {
		return "", fmt.Errorf("creating state dir %s: %w", root, err)
	}

	return root, nil
}

// readID returns the stored notification ID at path, or 0 on any failure
// (missing file, permission denied, unparseable contents). A zero return
// is the signal to the caller that there is no prior notification to
// replace — the next send will be a fresh one.
func readID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	id, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}

	return id
}

// writeID atomically writes id to path using a temp-file + rename.
// Safe against torn reads; a crash mid-write leaves either the old
// file or an orphan .tmp that readID ignores.
func writeID(path string, id int) error {
	tmp := path + ".tmp"
	err := os.WriteFile(tmp, []byte(strconv.Itoa(id)), 0o600)
	if err != nil {
		return fmt.Errorf("writing temp state file %s: %w", tmp, err)
	}

	err = os.Rename(tmp, path)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming state file to %s: %w", path, err)
	}

	return nil
}

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

	replaceKey := ""
	if n.ReplaceKey != "" {
		replaceKey, err = tmpl.Render("replace_key", n.ReplaceKey, tctx)
		if err != nil {
			return err
		}
	}

	statePath := ""
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
	if replaceKey != "" {
		dir, derr := stateDir()
		if derr != nil {
			slog.Warn("notify-send: state dir unavailable, sending without dedup", "err", derr)
		} else {
			statePath = filepath.Join(dir, dedupFilename(replaceKey))
			prev := readID(statePath)
			if prev > 0 {
				args = append(args, "-r", strconv.Itoa(prev))
			}
			args = append(args, "-p")
		}
	}
	args = append(args, title, body)

	cmd := exec.CommandContext(ctx, n.Path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("running %s: %s: %w", n.Path, stderr.String(), err)
	}

	if statePath != "" {
		out := strings.TrimSpace(stdout.String())
		newID, perr := strconv.Atoi(out)
		if perr != nil {
			slog.Debug("notify-send did not return a parseable id", "stdout", out)
		} else {
			werr := writeID(statePath, newID)
			if werr != nil {
				slog.Warn("notify-send: failed to persist id", "err", werr)
			}
		}
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

## Replace previous notifications from the same session (dedup).
## Go template — defaults to the Claude Code session ID so repeated
## notifications from one session update in place. Set to "" to disable.
## State lives under $XDG_RUNTIME_DIR/claude-notifier/notify-send/
## (or $TMPDIR/claude-notifier-<uid>/notify-send/ if unset).
# replace_key = "{{.SessionID}}"

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
