# ecs-tui

A terminal UI for Amazon ECS — monitoring & deployment tracking specialized. Browse clusters, services, and tasks interactively. View real-time logs with search, deployment dashboards with task definition diffs, CloudWatch metrics with sparklines, and task details. Execute commands in containers via ECS Exec. Manage deployments and scaling directly from the terminal.

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)

## Install

```bash
# Go
go install github.com/soominna/ecs-tui@latest

# Homebrew
brew tap soominna/tap
brew install ecs-tui

# GitHub Releases
# Download the binary for your platform from the Releases page.
```

## Usage

```bash
# Launch with interactive profile/region selection
ecs-tui

# Specify AWS profile and region
ecs-tui --profile my-profile --region ap-northeast-2

# Jump directly to a cluster
ecs-tui --cluster my-cluster

# Jump directly to a service's tasks
ecs-tui --cluster my-cluster --service my-service

# Read-only mode (blocks all mutative actions)
ecs-tui --read-only

# Enable CloudWatch metrics (default: off to avoid costs)
ecs-tui --metrics

# Custom auto-refresh interval (seconds, -1 to disable)
ecs-tui --refresh 30

# Print version
ecs-tui --version
```

## Configuration

Create `~/.config/ecs-tui/config.yml` to set defaults:

```yaml
# Default cluster and service to jump to on launch
default_cluster: my-cluster
default_service: my-service

# Auto-refresh interval in seconds (default: 10, -1 to disable)
refresh_interval: 10

# Start in read-only mode
read_only: false

# Shell to use for ECS Exec (default: /bin/sh)
shell: /bin/sh

# Color theme: mocha, latte
theme: mocha

# Enable CloudWatch metrics on startup (default: false)
# When disabled, no GetMetricData API calls are made ($0 cost)
metrics: false
```

CLI flags override config file values when explicitly provided.

## Features

- **Cluster → Service → Task** drill-down navigation with breadcrumb trail
- **Deployment dashboard** — view active deployments with progress bars, rollout state, and circuit breaker config (`D` key)
- **Task definition diff** — compare task definitions between deployments with colored diff output (`d` key in deployment view)
- **Real-time log streaming** via CloudWatch LiveTail with polling fallback
- **Log search** — search logs with `/`, navigate matches with `n`/`N`, log level coloring (ERROR, WARN, DEBUG)
- **Resource sparklines** — CPU/Memory utilization history as inline sparkline charts in the service table
- **CloudWatch metrics** — CPU/Memory utilization per service (default off, toggle with `m` key with cost warning)
- **Task detail** view with task definition, resource, and log configuration info
- **ECS Exec** — open a shell in a running container (configurable shell)
- **Service actions** — force new deployment, update desired count
- **Stop tasks** — stop individual tasks with confirmation
- **Task status filter** — toggle between Running, Stopped, and All tasks (`t` key)
- **Service events** — view recent deployment events
- **Read-only mode** — block all mutative actions with `--read-only` flag
- **Config file** — persist defaults in `~/.config/ecs-tui/config.yml`
- **AWS profile/region switching** at runtime (`P` key)
- **Session persistence** — remembers last used profile and region
- **Filtering** — search clusters, services, and tasks with `/`
- **Configurable auto-refresh** — default 10s, customizable or disable with `-1`
- **Keyboard-driven** — full TUI with help overlay (`?` key)

## Keybindings

### Global

| Key | Action |
|---|---|
| `P` | Change AWS profile/region |
| `T` | Toggle theme (mocha / latte) |
| `?` | Toggle help overlay |
| `Ctrl+C` | Quit |

### Cluster View

| Key | Action |
|---|---|
| `Enter` | Select cluster |
| `/` | Filter clusters |
| `r` | Refresh |
| `Esc` | Quit |

### Service View

| Key | Action |
|---|---|
| `Enter` | View tasks |
| `e` | View service events |
| `D` | View deployment dashboard |
| `f` | Force new deployment |
| `d` | Update desired count |
| `m` | Toggle CloudWatch metrics on/off |
| `/` | Filter services |
| `r` | Refresh (includes metrics if enabled) |
| `Esc` | Back to clusters |

### Deployment View

| Key | Action |
|---|---|
| `d` | View task definition diff (requires 2+ deployments) |
| `r` | Refresh |
| `j` / `k` | Scroll down / up |
| `Esc` | Back |

### Task View

| Key | Action |
|---|---|
| `Enter` / `d` | View task detail |
| `l` | View logs |
| `e` | Exec into container |
| `s` | Stop task |
| `t` | Toggle status filter (Running / Stopped / All) |
| `/` | Filter tasks |
| `r` | Refresh |
| `Esc` | Back to services |

### Log View

| Key | Action |
|---|---|
| `f` | Toggle follow mode |
| `/` | Search logs |
| `n` | Next search match |
| `N` | Previous search match |
| `G` | Jump to bottom |
| `g` | Jump to top |
| `j` / `k` | Scroll down / up |
| `Esc` | Clear search / Back |

### Detail / Events / Diff View

| Key | Action |
|---|---|
| `j` / `k` | Scroll down / up |
| `Esc` | Back |

## CloudWatch Metrics & Cost

CloudWatch metrics are **disabled by default** to avoid unexpected AWS costs. When disabled, no `GetMetricData` API calls are made.

To enable metrics:
- Press `m` in the service view (shows cost estimate before enabling)
- Or use `--metrics` flag / set `metrics: true` in config

When enabled, metrics are **only refreshed manually** via `r` key — no automatic polling. This keeps costs minimal even for large-scale usage.

| Scenario | Services | Metrics OFF (default) | Metrics ON (100 refreshes/day) |
|---|---|---|---|
| Small | 5 | $0.00/mo | ~$0.02/mo |
| Medium | 30 | $0.00/mo | ~$0.13/mo |
| Large | 100 | $0.00/mo | ~$0.44/mo |

## Requirements

- AWS credentials configured (`~/.aws/credentials` or environment variables)
- IAM permissions for ECS, CloudWatch Logs, and CloudWatch
- For ECS Exec: AWS CLI and [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

## License

MIT
