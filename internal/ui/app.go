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
	readOnly bool
	width    int
	height   int
	err      error
	status   string
	showHelp bool
	// Config
	refreshInterval time.Duration // negative = disabled, 0 = default (10s)
	shell           string        // shell for exec (default: /bin/sh)
}

func NewApp(client *awsclient.Client, cluster, service string, refreshInterval int, shell string, readOnly bool) *App {
	var interval time.Duration
	if refreshInterval < 0 {
		interval = -1
	} else if refreshInterval == 0 {
		interval = 10 * time.Second
	} else {
		interval = time.Duration(refreshInterval) * time.Second
	}

	if shell == "" {
		shell = "/bin/sh"
	}

	return &App{
		client:          client,
		cluster:         cluster,
		service:         service,
		readOnly:        readOnly,
		refreshInterval: interval,
		shell:           shell,
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
		view := NewTaskView(a.client, a.cluster, a.service, a.readOnly, a.refreshInterval, a.shell)
		a.stack = []View{view}
		return view.Init()
	}

	view := NewServiceView(a.client, a.cluster, a.readOnly, a.refreshInterval, a.shell)
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
			// Dismiss error if visible
			if a.err != nil {
				a.err = nil
				return a, nil
			}
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
		// Give more time for access denied errors
		if isAccessDenied(msg.Err) {
			return a, clearErrorAfter(15 * time.Second)
		}
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
		a.status = "Connecting to AWS..."
		a.err = nil
		profile := msg.Profile
		region := msg.Region
		return a, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
			defer cancel()
			client, err := awsclient.NewClient(ctx, profile, region)
			if err != nil {
				return awsClientErrorMsg{Err: err}
			}
			return awsClientReadyMsg{Client: client, Profile: profile, Region: region}
		}

	case awsClientReadyMsg:
		a.client = msg.Client
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


	case awsClientErrorMsg:
		a.err = msg.Err
		a.status = ""
		return a, clearErrorAfter(10 * time.Second)

	case ClusterSelectedMsg:
		a.cluster = msg.ClusterName
		serviceView := NewServiceView(a.client, a.cluster, a.readOnly, a.refreshInterval, a.shell)
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

	if a.readOnly {
		infoParts = append(infoParts, lipgloss.NewStyle().Foreground(colorPeach).Bold(true).Render("[READ-ONLY]"))
	}

	infoText := strings.Join(infoParts, "\n")
	if a.err != nil {
		errMsg := fmt.Sprintf("ERR: %v", a.err)
		if isAccessDenied(a.err) {
			errMsg += " (check IAM permissions: ecs:List*, ecs:Describe*, logs:*, cloudwatch:GetMetricData)"
		}
		infoText += "\n" + errorStyle.Render(errMsg)
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

// layoutOverhead is the total vertical lines used by header, breadcrumb, footer, and separators.
// logo header (4 lines) + breadcrumb(1) + footer(1) + newlines(2) = 8
const layoutOverhead = 8

func (a *App) contentHeight() int {
	return a.height - layoutOverhead
}

// isAccessDenied checks if the error is an AWS IAM permission error.
func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "AccessDeniedException") ||
		strings.Contains(msg, "AccessDenied") ||
		strings.Contains(msg, "UnauthorizedAccess") ||
		strings.Contains(msg, "is not authorized to perform")
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
