# ecs-tui

A terminal UI for Amazon ECS. Browse clusters, services, and tasks interactively. View real-time logs, CloudWatch metrics, and task details. Execute commands in containers via ECS Exec. Manage deployments and scaling directly from the terminal.

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

# Print version
ecs-tui --version
```

## Features

- **Cluster → Service → Task** drill-down navigation with breadcrumb trail
- **Real-time log streaming** via CloudWatch LiveTail with polling fallback
- **Task detail** view with task definition, resource, and log configuration info
- **ECS Exec** — open a shell in a running container directly from the TUI
- **Service actions** — force new deployment, update desired count
- **Stop tasks** — stop individual tasks with confirmation
- **CloudWatch metrics** — CPU/Memory utilization per service
- **Service events** — view recent deployment events
- **AWS profile/region switching** at runtime (`P` key)
- **Session persistence** — remembers last used profile and region
- **Filtering** — search clusters, services, and tasks with `/`
- **Auto-refresh** — services and tasks update every 10 seconds
- **Keyboard-driven** — full TUI with help overlay (`?` key)

## Keybindings

### Global

| Key | Action |
|---|---|
| `P` | Change AWS profile/region |
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
| `f` | Force new deployment |
| `d` | Update desired count |
| `/` | Filter services |
| `r` | Refresh |
| `Esc` | Back to clusters |

### Task View

| Key | Action |
|---|---|
| `Enter` / `d` | View task detail |
| `l` | View logs |
| `e` | Exec into container |
| `s` | Stop task |
| `/` | Filter tasks |
| `r` | Refresh |
| `Esc` | Back to services |

### Log View

| Key | Action |
|---|---|
| `f` | Toggle follow mode |
| `G` | Jump to bottom |
| `g` | Jump to top |
| `j` / `k` | Scroll down / up |
| `Esc` | Back |

### Detail / Events View

| Key | Action |
|---|---|
| `j` / `k` | Scroll down / up |
| `Esc` | Back |

## Requirements

- AWS credentials configured (`~/.aws/credentials` or environment variables)
- IAM permissions for ECS, CloudWatch Logs, and CloudWatch
- For ECS Exec: AWS CLI and [Session Manager Plugin](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html)

## License

MIT
