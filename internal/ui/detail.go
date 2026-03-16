package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type DetailView struct {
	client   *awsclient.Client
	task     *awsclient.TaskInfo
	taskDef  *awsclient.TaskDefinitionInfo
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

type taskDefDetailMsg struct {
	def *awsclient.TaskDefinitionInfo
}

func NewDetailView(client *awsclient.Client, task *awsclient.TaskInfo) *DetailView {
	return &DetailView{
		client: client,
		task:   task,
	}
}

func (v *DetailView) Title() string { return "Task Detail" }

func (v *DetailView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "j/k", Desc: "Scroll"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *DetailView) Init() tea.Cmd {
	if v.task.TaskDefARN == "" {
		return nil
	}
	return func() tea.Msg {
		td, err := v.client.DescribeTaskDefinition(context.Background(), v.task.TaskDefARN)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return taskDefDetailMsg{def: td}
	}
}

func (v *DetailView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if !v.ready {
			v.viewport = viewport.New(v.width, v.height)
			v.ready = true
		} else {
			v.viewport.Width = v.width
			v.viewport.Height = v.height
		}
		v.viewport.SetContent(v.renderContent())
		return v, nil

	case taskDefDetailMsg:
		v.taskDef = msg.def
		if v.ready {
			v.viewport.SetContent(v.renderContent())
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return v, func() tea.Msg { return PopViewMsg{} }
		}
	}

	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *DetailView) View() string {
	if !v.ready {
		return loadingStyle.Render("  Loading details...")
	}
	return v.viewport.View()
}

func (v *DetailView) renderContent() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(colorAccent).Width(20)
	valueStyle := lipgloss.NewStyle().Foreground(colorWhite)

	var sb strings.Builder

	sb.WriteString(sectionTitleStyle.Render("Task Detail"))
	sb.WriteString("\n")

	fields := []struct {
		label string
		value string
	}{
		{"Task ID", v.task.TaskID},
		{"Task ARN", v.task.TaskARN},
		{"Status", v.task.Status},
		{"IP Address", v.task.IP},
		{"Health Status", v.task.HealthStatus},
		{"Container", v.task.ContainerName},
		{"Task Definition", v.task.TaskDefARN},
	}

	if v.task.StartedAt != nil {
		fields = append(fields, struct {
			label string
			value string
		}{"Started At", v.task.StartedAt.Local().Format("2006-01-02 15:04:05 MST")})
	}

	for _, f := range fields {
		val := f.value
		if val == "" {
			val = "-"
		}
		if f.label == "Status" {
			val = StatusColor(val)
		}
		sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render(f.label+":"), valueStyle.Render(val)))
	}

	if v.taskDef != nil {
		sb.WriteString("\n")
		sb.WriteString(sectionTitleStyle.Render("Task Definition"))
		sb.WriteString("\n")

		tdFields := []struct {
			label string
			value string
		}{
			{"Family", v.taskDef.Family},
			{"CPU", v.taskDef.CPU},
			{"Memory", v.taskDef.Memory},
			{"Log Group", v.taskDef.LogGroup},
			{"Log Prefix", v.taskDef.LogPrefix},
		}

		for _, f := range tdFields {
			val := f.value
			if val == "" {
				val = "-"
			}
			sb.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render(f.label+":"), valueStyle.Render(val)))
		}
	}

	return sb.String()
}
