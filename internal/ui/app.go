package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
	execpkg "github.com/soominna/ecs-tui/internal/exec"
)

// App is the root model that manages the view stack.
type App struct {
	stack    []View
	client   *awsclient.Client
	cluster  string
	service  string
	width    int
	height   int
	err      error
	status   string
	showHelp bool
}

func NewApp(client *awsclient.Client, cluster, service string) *App {
	return &App{
		client:  client,
		cluster: cluster,
		service: service,
	}
}

func (a *App) Init() tea.Cmd {
	if a.client == nil {
		configView := NewConfigView()
		a.stack = []View{configView}
		return configView.Init()
	}

	if a.cluster == "" {
		view := NewClusterView(a.client)
		a.stack = []View{view}
		return view.Init()
	}

	if a.service != "" {
		view := NewTaskView(a.client, a.cluster, a.service)
		a.stack = []View{view}
		return view.Init()
	}

	view := NewServiceView(a.client, a.cluster)
	a.stack = []View{view}
	return view.Init()
}

// closeView calls Close() on a view if it implements Closeable.
func closeView(v View) {
	if c, ok := v.(Closeable); ok {
		c.Close()
	}
}

// updateCurrentView safely updates the current view on the stack.
func (a *App) updateCurrentView(updated tea.Model) {
	if len(a.stack) == 0 {
		return
	}
	if v, ok := updated.(View); ok {
		a.stack[len(a.stack)-1] = v
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if len(a.stack) > 0 {
			contentHeight := a.contentHeight()
			resizeMsg := tea.WindowSizeMsg{Width: a.width, Height: contentHeight}
			current := a.stack[len(a.stack)-1]
			updated, cmd := current.Update(resizeMsg)
			a.updateCurrentView(updated)
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Cleanup all views before quitting
			for _, v := range a.stack {
				closeView(v)
			}
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "esc":
			if a.showHelp {
				a.showHelp = false
				return a, nil
			}
			// Fall through to current view's esc handler
		// Theme toggle disabled temporarily
		// case "T", "t":
		//	ToggleTheme()
		//  ...
		case "P", "p":
			if a.client != nil {
				configView := NewConfigViewWithCurrent(a.client.Profile, a.client.Region)
				return a, func() tea.Msg {
					return PushViewMsg{View: configView}
				}
			}
		}

	case PushViewMsg:
		a.showHelp = false
		a.stack = append(a.stack, msg.View)
		contentHeight := a.contentHeight()
		resizeMsg := tea.WindowSizeMsg{Width: a.width, Height: contentHeight}
		updated, resizeCmd := msg.View.Update(resizeMsg)
		a.updateCurrentView(updated)
		initCmd := msg.View.Init()
		return a, tea.Batch(initCmd, resizeCmd)

	case PopViewMsg:
		if len(a.stack) <= 1 {
			for _, v := range a.stack {
				closeView(v)
			}
			return a, tea.Quit
		}
		// Close the popped view
		closeView(a.stack[len(a.stack)-1])
		a.stack = a.stack[:len(a.stack)-1]
		if len(a.stack) > 0 {
			contentHeight := a.contentHeight()
			resizeMsg := tea.WindowSizeMsg{Width: a.width, Height: contentHeight}
			current := a.stack[len(a.stack)-1]
			updated, cmd := current.Update(resizeMsg)
			a.updateCurrentView(updated)
			return a, cmd
		}
		return a, nil

	case ErrorMsg:
		a.err = msg.Err
		return a, clearErrorAfter(5 * time.Second)

	case ClearErrorMsg:
		a.err = nil
		return a, nil

	case StatusMsg:
		a.status = msg.Message
		return a, nil

	case execpkg.ExecDoneMsg:
		if msg.Err != nil {
			if msg.Hint != "" {
				hintView := NewExecHintView(msg.Err.Error(), msg.Hint)
				return a, func() tea.Msg {
					return PushViewMsg{View: hintView}
				}
			}
			a.err = msg.Err
			return a, clearErrorAfter(15 * time.Second)
		}
		return a, nil

	case AWSConfigChangedMsg:
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client, err := awsclient.NewClient(ctx, msg.Profile, msg.Region)
		if err != nil {
			a.err = err
			return a, clearErrorAfter(5 * time.Second)
		}
		a.client = client
		// Best-effort session save; don't block on failure
		_ = awsclient.SaveLastSession(msg.Profile, msg.Region, CurrentThemeName()) //nolint:errcheck
		a.cluster = ""
		a.service = ""
		a.err = nil
		a.status = ""
		// Close all existing views
		for _, v := range a.stack {
			closeView(v)
		}
		clusterView := NewClusterView(a.client)
		a.stack = []View{clusterView}
		return a, clusterView.Init()

	case ClusterSelectedMsg:
		a.cluster = msg.ClusterName
		serviceView := NewServiceView(a.client, a.cluster)
		return a, func() tea.Msg {
			return PushViewMsg{View: serviceView}
		}
	}

	// Forward to current view
	if len(a.stack) > 0 {
		current := a.stack[len(a.stack)-1]
		updated, cmd := current.Update(msg)
		a.updateCurrentView(updated)
		return a, cmd
	}

	return a, nil
}

func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	header := a.renderHeader()
	footer := a.renderFooter()

	var content string
	if a.showHelp && len(a.stack) > 0 {
		content = renderHelpOverlay(a.stack[len(a.stack)-1], a.width, a.contentHeight())
	} else if len(a.stack) > 0 {
		content = a.stack[len(a.stack)-1].View()
	}

	// Pad content to fill space with themed background
	contentHeight := a.contentHeight()
	contentLines := countLines(content)
	if contentLines < contentHeight {
		content += strings.Repeat("\n", contentHeight-contentLines)
	}

	return header + "\n" + content + "\n" + footer
}

func (a *App) renderHeader() string {
	logoStyle := lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	logo := logoStyle.Render(logoText)

	// Build info text
	var infoParts []string
	if a.client != nil {
		if a.client.Profile != "" {
			infoParts = append(infoParts, fmt.Sprintf("Profile: %s", a.client.Profile))
		}
		if a.client.Region != "" {
			infoParts = append(infoParts, fmt.Sprintf("Region: %s", a.client.Region))
		}
	}
	if a.cluster != "" {
		infoParts = append(infoParts, fmt.Sprintf("Cluster: %s", a.cluster))
	}

	infoText := strings.Join(infoParts, "\n")
	if a.err != nil {
		infoText += "\n" + errorStyle.Render(fmt.Sprintf("ERR: %v", a.err))
	} else if a.status != "" {
		infoText += "\n" + statusStyle.Render(a.status)
	}

	infoStyle := lipgloss.NewStyle().Foreground(colorSubtext1).PaddingLeft(6)
	header := lipgloss.JoinHorizontal(lipgloss.Top, logo, infoStyle.Render(infoText))
	header = headerStyle.Width(a.width).Render(header)

	// Breadcrumb navigation path
	if len(a.stack) > 0 {
		header += "\n" + a.renderBreadcrumb()
	}

	return header
}

func (a *App) renderBreadcrumb() string {
	var crumbs []string
	for i, v := range a.stack {
		title := v.Title()
		if i == len(a.stack)-1 {
			crumbs = append(crumbs, breadcrumbActiveStyle.Render(title))
		} else {
			crumbs = append(crumbs, breadcrumbStyle.Render(title))
		}
	}
	sep := breadcrumbSepStyle.Render(" > ")
	return " " + strings.Join(crumbs, sep)
}

func (a *App) renderFooter() string {
	var shortcuts []Shortcut

	if len(a.stack) > 0 && !a.showHelp {
		shortcuts = a.stack[len(a.stack)-1].ShortcutHelp()
	}

	if a.client != nil {
		shortcuts = append(shortcuts, Shortcut{Key: "P", Desc: "Profile"})
	}
	shortcuts = append(shortcuts,
		Shortcut{Key: "?", Desc: "Help"},
		Shortcut{Key: "Ctrl+C", Desc: "Quit"},
	)

	return RenderShortcutBar(shortcuts, a.width)
}

func (a *App) contentHeight() int {
	// logo header (4 lines) + breadcrumb(1) + footer(1) + newlines(2) = 8
	return a.height - 8
}

func clearErrorAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return ClearErrorMsg{}
	})
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
