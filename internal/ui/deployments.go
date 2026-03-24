package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type DeploymentView struct {
	client          awsclient.ECSAPI
	cluster         string
	serviceName     string
	deployInfo    *awsclient.ServiceDeploymentInfo
	vh            viewportHelper
	width, height int
	lastUpdated     time.Time
	refreshInterval time.Duration
}

type deploymentLoadedMsg struct {
	info *awsclient.ServiceDeploymentInfo
}

type deploymentTickMsg time.Time

func NewDeploymentView(client awsclient.ECSAPI, cluster, serviceName string, refreshInterval time.Duration) *DeploymentView {
	return &DeploymentView{
		client:          client,
		cluster:         cluster,
		serviceName:     serviceName,
		refreshInterval: refreshInterval,
	}
}

func (v *DeploymentView) Title() string {
	return fmt.Sprintf("Deployments (%s)", v.serviceName)
}

func (v *DeploymentView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "r", Desc: "Refresh"},
		{Key: "d", Desc: "Diff"},
		{Key: "j/k", Desc: "Scroll"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *DeploymentView) Init() tea.Cmd {
	return tea.Batch(v.fetchDeployments(), v.tickCmd())
}

func (v *DeploymentView) tickCmd() tea.Cmd {
	return newTickCmd(v.refreshInterval, func(t time.Time) deploymentTickMsg { return deploymentTickMsg(t) })
}

func (v *DeploymentView) fetchDeployments() tea.Cmd {
	client := v.client
	cluster := v.cluster
	svcName := v.serviceName
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		info, err := client.GetServiceDeployments(ctx, cluster, svcName)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return deploymentLoadedMsg{info: info}
	}
}

func (v *DeploymentView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.vh.handleResize(v.width, v.height)
		v.vh.viewport.SetContent(v.renderContent())
		return v, nil

	case deploymentLoadedMsg:
		v.deployInfo = msg.info
		v.lastUpdated = time.Now()
		if v.vh.ready {
			v.vh.viewport.SetContent(v.renderContent())
		}
		return v, nil

	case deploymentTickMsg:
		return v, tea.Batch(v.fetchDeployments(), v.tickCmd())

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return v, func() tea.Msg { return PopViewMsg{} }
		case "r":
			return v, v.fetchDeployments()
		case "d":
			// Diff: need at least 2 deployments with different task defs
			if v.deployInfo != nil && len(v.deployInfo.Deployments) >= 2 {
				oldTD := v.deployInfo.Deployments[1].TaskDefinitionFull
				newTD := v.deployInfo.Deployments[0].TaskDefinitionFull
				if oldTD != newTD {
					diffView := NewTaskDefDiffView(v.client, oldTD, newTD,
						v.deployInfo.Deployments[1].TaskDefinition,
						v.deployInfo.Deployments[0].TaskDefinition)
					return v, func() tea.Msg {
						return PushViewMsg{View: diffView}
					}
				}
			}
			return v, nil
		}
	}

	if v.vh.ready {
		var cmd tea.Cmd
		v.vh.viewport, cmd = v.vh.viewport.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *DeploymentView) View() string {
	if !v.vh.ready {
		return loadingStyle.Render("  Loading deployments...")
	}
	return v.vh.viewport.View()
}

func renderProgress(running, desired int32) string {
	if desired == 0 {
		return "0/0"
	}
	pct := float64(running) / float64(desired)
	filled := int(pct * 10)
	if filled > 10 {
		filled = 10
	}
	if filled < 0 {
		filled = 0
	}
	return fmt.Sprintf("[%s%s] %d/%d",
		strings.Repeat("█", filled),
		strings.Repeat("░", 10-filled),
		running, desired)
}

func (v *DeploymentView) renderContent() string {
	if v.deployInfo == nil {
		return loadingStyle.Render("  Loading deployments...")
	}

	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(colorBlue).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(colorText)
	dimStyle := lipgloss.NewStyle().Foreground(colorSubtext0)

	var sb strings.Builder

	// Updated time
	if !v.lastUpdated.IsZero() {
		ago := time.Since(v.lastUpdated).Truncate(time.Second)
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(
			fmt.Sprintf("  Updated %s ago", ago)))
		sb.WriteString("\n\n")
	}

	// Deployment Config
	sb.WriteString(sectionTitleStyle.Render("Deployment Config"))
	sb.WriteString("\n")
	dc := v.deployInfo.DeploymentConfig
	sb.WriteString(fmt.Sprintf("  %s %s\n",
		labelStyle.Render("Strategy:"),
		valueStyle.Render(fmt.Sprintf("Min %d%% / Max %d%%", dc.MinimumHealthyPercent, dc.MaximumPercent))))

	cbStatus := "Disabled"
	if dc.CircuitBreakerEnabled {
		rollback := "Off"
		if dc.CircuitBreakerRollback {
			rollback = "On"
		}
		cbStatus = fmt.Sprintf("Enabled (Rollback: %s)", rollback)
	}
	sb.WriteString(fmt.Sprintf("  %s %s\n",
		labelStyle.Render("Circuit Breaker:"),
		valueStyle.Render(cbStatus)))

	sb.WriteString("\n")
	sb.WriteString(sectionTitleStyle.Render("Active Deployments"))
	sb.WriteString("\n")

	for _, d := range v.deployInfo.Deployments {
		// Status with color
		statusStr := d.Status
		switch d.Status {
		case "PRIMARY":
			statusStr = lipgloss.NewStyle().Foreground(colorGreen).Bold(true).Render(d.Status)
		case "ACTIVE":
			statusStr = lipgloss.NewStyle().Foreground(colorBlue).Render(d.Status)
		default:
			statusStr = lipgloss.NewStyle().Foreground(colorSubtext0).Render(d.Status)
		}

		progress := renderProgress(d.RunningCount, d.DesiredCount)

		// Rollout state color
		rolloutStr := d.RolloutState
		switch d.RolloutState {
		case "COMPLETED":
			rolloutStr = lipgloss.NewStyle().Foreground(colorGreen).Render("COMPLETED")
		case "IN_PROGRESS":
			rolloutStr = lipgloss.NewStyle().Foreground(colorBlue).Render("IN_PROGRESS")
		case "FAILED":
			rolloutStr = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("FAILED")
		}

		sb.WriteString(fmt.Sprintf("\n  %s  %s  %s  %s\n",
			statusStr,
			valueStyle.Render(d.TaskDefinition),
			progress,
			rolloutStr))

		if d.CreatedAt != nil {
			sb.WriteString(fmt.Sprintf("    %s %s",
				dimStyle.Render("Created:"),
				dimStyle.Render(d.CreatedAt.Local().Format("2006-01-02 15:04:05"))))
		}
		if d.UpdatedAt != nil {
			ago := time.Since(*d.UpdatedAt).Truncate(time.Second)
			sb.WriteString(fmt.Sprintf("  %s %s",
				dimStyle.Render("Updated:"),
				dimStyle.Render(fmt.Sprintf("%s ago", ago))))
		}
		sb.WriteString("\n")

		if d.FailedTasks > 0 {
			sb.WriteString(fmt.Sprintf("    %s\n",
				lipgloss.NewStyle().Foreground(colorRed).Render(fmt.Sprintf("Failed: %d", d.FailedTasks))))
		}
		if d.PendingCount > 0 {
			sb.WriteString(fmt.Sprintf("    %s\n",
				dimStyle.Render(fmt.Sprintf("Pending: %d", d.PendingCount))))
		}
		if d.RolloutStateReason != "" {
			sb.WriteString(fmt.Sprintf("    %s\n",
				dimStyle.Render(d.RolloutStateReason)))
		}
	}

	if len(v.deployInfo.Deployments) >= 2 {
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("  Press 'd' to view task definition diff"))
		sb.WriteString("\n")
	}

	return sb.String()
}
