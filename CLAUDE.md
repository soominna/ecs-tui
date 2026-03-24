# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ecs-tui is a terminal UI for Amazon ECS built in Go 1.24. It uses Bubble Tea (bubbletea) for the TUI framework with Catppuccin theming (mocha/latte). Users can browse clusters, services, and tasks; stream logs; view deployments with task definition diffs; see CloudWatch metrics with sparklines; and exec into containers.

## Build & Development Commands

```bash
# Build binary
make build                    # produces ./ecs-tui

# Build with version
VERSION=1.0.0 make build

# Run
go run .
go run . --profile my-profile --region us-east-1

# Test
go test ./...                 # all tests
go test ./internal/aws/...    # aws package only
go test ./internal/ui/...     # ui package only
go test -run TestDiff ./internal/aws/  # single test

# Vet (CI check)
go vet ./...

# Release dry run (requires goreleaser)
make release-dry
```

## CI

CI runs `go build ./...` and `go vet ./...` on push/PR to main. There are no lint or test steps in CI currently — tests are run locally.

## Architecture

### Package Structure

- `main.go` — CLI flag parsing, config loading, AWS client init, launches Bubble Tea program
- `internal/aws/` — AWS SDK v2 wrapper (ECS, CloudWatch Logs, CloudWatch Metrics)
- `internal/ui/` — All Bubble Tea views and rendering
- `internal/config/` — YAML config file loading (`~/.config/ecs-tui/config.yml`)
- `internal/exec/` — ECS Exec subprocess management (shells out to `aws ecs execute-command`)

### UI View Stack Architecture

The app uses a **view stack** pattern (`App.stack []View`). All views implement the `View` interface (`internal/ui/common.go`):

```go
type View interface {
    tea.Model
    ShortcutHelp() []Shortcut
    Title() string
}
```

Navigation uses message-passing: `PushViewMsg` pushes a view, `PopViewMsg` pops. Views that hold resources (goroutines, streams) implement the `Closeable` interface and get `Close()` called on pop.

**View hierarchy:** ConfigView → ClusterView → ServiceView → TaskView → (LogView | DetailView | DeploymentView | DiffView | EventsView | ExecHintView)

The `App` model in `app.go` handles global concerns: header/breadcrumb/footer rendering, error display with auto-clear, theme toggling, AWS profile switching, and routing messages to the current top-of-stack view.

### AWS Client Layer

`internal/aws/client.go` wraps three SDK clients (ECS, CloudWatch Logs, CloudWatch). The `ECSAPI` interface (`iface.go`) abstracts all client methods — the UI layer depends only on this interface. Tests use `MockECSAPI` (`mock_test.go`) with function fields.

Key patterns:
- All AWS API calls respect pagination and batch size limits (DescribeClusters: 100, DescribeServices: 10, DescribeTasks: 100, GetMetricData: 500)
- Parallel fetches use `errgroup` or manual goroutines with semaphore channels (concurrency capped at 5)
- Task definition results are cached at the App level (`taskDefCache`) and shared via `taskDefCacheUpdateMsg`
- CloudWatch metrics are off by default to avoid costs; toggled via `m` key with cost confirmation

### Async Data Flow

Views fetch data via `tea.Cmd` functions that run AWS calls in goroutines, returning typed messages (e.g., `servicesLoadedMsg`, `taskDefsLoadedMsg`). The pattern is: fetch → return msg → `Update()` stores data → calls `rebuildTable()`. Auto-refresh uses `tea.Tick` with configurable interval.

### Theming

Uses Catppuccin palette. Colors are package-level vars in `styles.go`, set by `ApplyTheme()` in `theme.go`. All styles reference these vars. Two themes: mocha (dark) and latte (light).

### Session & Config Persistence

- Config: `~/.config/ecs-tui/config.yml` (YAML, loaded via `internal/config/`)
- Session: `~/.config/ecs-tui/session.json` (last profile/region/theme, atomic write with temp file)

## Key Conventions

- AWS API timeout is 30s (`apiTimeout` constant in `common.go`)
- All mutative actions (force deploy, update count, stop task) are blocked in read-only mode
- ECS Exec validates args against injection (rejects values starting with `-`)
- Overlay modals (confirm dialogs, count input) use `RenderOverlay()` compositing in `styles.go`
- Responsive table columns use flex-based width distribution (`responsiveColumn` / `calcColumnWidths`)
- Log streaming uses CloudWatch LiveTail with GetLogEvents polling as fallback
