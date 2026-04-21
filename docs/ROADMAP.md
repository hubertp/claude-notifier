# Roadmap

## Features

- [ ] `claude-notifier list` command — show all registered plugins and their sample config
- [ ] Config validation on startup — warn about unknown keys or missing required fields
- [ ] Quiet mode (`-q`) — suppress slog output for clean hook usage
- [ ] Per-plugin timeout override

## Plugins

### notify-send (Linux desktop notifications)

- [x] `notify-send` wrapper for native Linux desktop notifications
- [x] Configurable urgency level (low, normal, critical; auto-maps from NotificationType)
- [x] Configurable icon
- [x] Configurable expiration timeout
- [x] Same-session dedup via `-r` / `-p`

### Slack webhook

- [ ] POST to Slack incoming webhook URL
- [ ] Configurable message format (markdown)
- [ ] Configurable channel override

### Discord webhook

- [ ] POST to Discord webhook URL
- [ ] Configurable embed format
- [ ] Configurable username/avatar override

### Pushover

- [ ] Push notifications via Pushover API
- [ ] Configurable priority and sound
- [ ] Configurable device targeting

### Sound

- [ ] Play a sound on notification (platform-specific)
- [ ] Configurable sound file path
