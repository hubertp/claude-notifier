package main

import (
	"log/slog"
	"os"

	appcli "github.com/felipeelias/claude-notifier/internal/cli"
	"github.com/felipeelias/claude-notifier/internal/notifier"
	"github.com/felipeelias/claude-notifier/plugins/notifysend"
	"github.com/felipeelias/claude-notifier/plugins/ntfy"
	"github.com/felipeelias/claude-notifier/plugins/terminalnotifier"
)

var version = "dev"

func main() {
	reg := notifier.NewRegistry()
	ntfy.Register(reg)
	notifysend.Register(reg)
	terminalnotifier.Register(reg)

	app := appcli.New(version, reg)

	err := app.Run(os.Args)
	if err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
