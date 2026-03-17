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

type ServiceEventsView struct {
	client      *awsclient.Client
	cluster     string
	serviceName string
	events      []awsclient.ServiceEvent
	viewport    viewport.Model
	width       int
	height      int
	ready       bool
}

type eventsLoadedMsg struct {
	events []awsclient.ServiceEvent
}

func NewServiceEventsView(client *awsclient.Client, cluster, serviceName string) *ServiceEventsView {
	return &ServiceEventsView{
		client:      client,
		cluster:     cluster,
		serviceName: serviceName,
	}
}

func (v *ServiceEventsView) Title() string {
	return fmt.Sprintf("Events (%s)", v.serviceName)
}

func (v *ServiceEventsView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "j/k", Desc: "Scroll"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *ServiceEventsView) Init() tea.Cmd {
	client := v.client
	cluster := v.cluster
	serviceName := v.serviceName
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		events, err := client.GetServiceEvents(ctx, cluster, serviceName)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return eventsLoadedMsg{events: events}
	}
}

func (v *ServiceEventsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

	case themeChangedMsg:
		if v.ready {
			v.viewport.SetContent(v.renderContent())
		}
		return v, nil

	case eventsLoadedMsg:
		v.events = msg.events
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

func (v *ServiceEventsView) View() string {
	if !v.ready {
		return loadingStyle.Render("  Loading events...")
	}
	return v.viewport.View()
}

func (v *ServiceEventsView) renderContent() string {
	if len(v.events) == 0 {
		return loadingStyle.Render("  Loading events...")
	}

	timeStyle := lipgloss.NewStyle().Foreground(colorBlue).Width(22)
	msgStyle := lipgloss.NewStyle().Foreground(colorSubtext1)

	var sb strings.Builder
	sb.WriteString(sectionTitleStyle.Render(fmt.Sprintf("Service Events — %s", v.serviceName)))
	sb.WriteString("\n")

	for _, e := range v.events {
		ts := e.CreatedAt.Local().Format("2006-01-02 15:04:05")
		sb.WriteString(fmt.Sprintf("  %s %s\n", timeStyle.Render(ts), msgStyle.Render(e.Message)))
	}

	return sb.String()
}
