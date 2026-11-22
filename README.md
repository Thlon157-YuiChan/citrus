# Citrus

TUI/GUI client for uwuzu, with a pluggable backend design for future
Mastodon and Misskey support.

## Stack

- Language: Go
- TUI: bubbletea + lipgloss
- GUI: miqt (Qt6 bindings)
- Persistence: SQLite (modernc.org/sqlite)
- Backend: uwuzu first, then Mastodon/Misskey via `core.Backend`

## Layout

- `cmd/citrus` entry point (`-mode tui|gui`)
- `internal/core` domain + backend interface
- `internal/backend/uwuzu` uwuzu adapter
- `internal/tui` TUI (bubbletea)
- `internal/gui` GUI (miqt/Qt6)
- `internal/config`, `internal/storage`

## Build

```sh
go build ./...
go run ./cmd/citrus -mode tui
go run ./cmd/citrus -mode gui
```
