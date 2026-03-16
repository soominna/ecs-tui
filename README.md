# ecs-tui

A terminal UI for Amazon ECS. Browse clusters, services, and tasks interactively. View real-time logs, CloudWatch metrics, and task details. Quickly copy ECS Exec commands for container access. Supports AWS profile and region switching.

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

# Jump directly to a service
ecs-tui --cluster my-cluster --service my-service

# Print version
ecs-tui --version
```

## Features

- **Cluster → Service → Task** drill-down navigation with breadcrumb
- **Real-time log streaming** via CloudWatch LiveTail
- **Task detail** view with task definition info
- **ECS Exec** command copy for container shell access
- **CloudWatch metrics** (CPU, Memory) per service
- **AWS profile/region switching** at runtime (`P` key)
- **Session persistence** — remembers last used profile and region
- **Keyboard-driven** — full TUI with help overlay (`?` key)

## Keybindings

| Key | Action |
|---|---|
| `Enter` | Select / drill down |
| `Esc` | Go back |
| `P` | Change AWS profile/region |
| `/` | Filter/search |
| `?` | Toggle help |
| `Ctrl+C` | Quit |

## Requirements

- AWS credentials configured (`~/.aws/credentials` or environment variables)
- Appropriate IAM permissions for ECS, CloudWatch Logs, and CloudWatch

## License

MIT
