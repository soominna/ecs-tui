package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

const maxLogLines = 10000

type LogView struct {
	client     awsclient.ECSAPI
	cluster    string
	task       *awsclient.TaskInfo
	viewport   viewport.Model
	logLines   []string
	width      int
	height     int
	ready      bool
	autoScroll bool
	streaming  bool
	logInfo    *awsclient.LogInfo
	closed     bool

	// LiveTail
	eventCh chan awsclient.LogEvent
	cancel  context.CancelFunc

	// Polling fallback
	nextToken  string
	polling    bool
	pollCtx    context.Context
	pollCancel context.CancelFunc

	// Search
	searchInput   textinput.Model
	searching     bool
	searchText    string
	searchMatches []int // indices into logLines
	searchCurrent int   // current match position
}

type logInfoMsg struct {
	info *awsclient.LogInfo
}

type logEventMsg struct {
	event awsclient.LogEvent
}

type logStreamEndMsg struct{}

type liveTailStartedMsg struct {
	eventCh chan awsclient.LogEvent
	cancel  context.CancelFunc
}

type liveTailFailedMsg struct {
	err error
}

type logPollTickMsg time.Time

type logEventsMsg struct {
	events    []awsclient.LogEvent
	nextToken string
}

func NewLogView(client awsclient.ECSAPI, cluster string, task *awsclient.TaskInfo) *LogView {
	si := textinput.New()
	si.Placeholder = "Search logs..."
	si.CharLimit = 100

	return &LogView{
		client:      client,
		cluster:     cluster,
		task:        task,
		autoScroll:  true,
		searchInput: si,
	}
}

func (v *LogView) Title() string {
	id := v.task.TaskID
	if len(id) > 12 {
		id = id[:12]
	}
	return fmt.Sprintf("Logs (%s)", id)
}

func (v *LogView) ShortcutHelp() []Shortcut {
	if v.searching {
		return []Shortcut{
			{Key: "Enter", Desc: "Search"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	followDesc := "Follow (off)"
	if v.autoScroll {
		followDesc = "Follow (on)"
	}
	shortcuts := []Shortcut{
		{Key: "f", Desc: followDesc},
		{Key: "/", Desc: "Search"},
	}
	if v.searchText != "" {
		shortcuts = append(shortcuts, Shortcut{Key: "n/N", Desc: "Next/Prev"})
	}
	shortcuts = append(shortcuts,
		Shortcut{Key: "G", Desc: "Bottom"},
		Shortcut{Key: "g", Desc: "Top"},
		Shortcut{Key: "Esc", Desc: "Back"},
	)
	return shortcuts
}

func (v *LogView) Init() tea.Cmd {
	return v.fetchLogInfo()
}

func (v *LogView) fetchLogInfo() tea.Cmd {
	client := v.client
	taskDefARN := v.task.TaskDefARN
	containerName := v.task.ContainerName
	taskID := v.task.TaskID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		info, err := client.GetLogInfo(ctx, taskDefARN, containerName, taskID)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return logInfoMsg{info: info}
	}
}

func (v *LogView) startLiveTail() tea.Cmd {
	client := v.client
	logGroupARN := v.logInfo.LogGroupARN
	logStream := v.logInfo.LogStream

	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		eventCh := make(chan awsclient.LogEvent, 100)

		streamNames := []string{}
		if logStream != "" {
			streamNames = []string{logStream}
		}

		err := client.StartLiveTail(ctx, logGroupARN, streamNames, "", eventCh)
		if err != nil {
			cancel()
			return liveTailFailedMsg{err: err}
		}
		return liveTailStartedMsg{eventCh: eventCh, cancel: cancel}
	}
}

func waitForLogEvent(ch <-chan awsclient.LogEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return logStreamEndMsg{}
		}
		return logEventMsg{event: event}
	}
}

func (v *LogView) startPolling() tea.Cmd {
	v.polling = true
	// Create a cancellable context for polling; cancelled by Close()
	ctx, cancel := context.WithCancel(context.Background())
	v.pollCtx = ctx
	v.pollCancel = cancel
	return tea.Batch(v.pollLogsWithCtx(ctx), v.pollTickCmd())
}

func (v *LogView) pollTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return logPollTickMsg(t)
	})
}

func (v *LogView) pollLogsWithCtx(parentCtx context.Context) tea.Cmd {
	client := v.client
	logGroup := v.logInfo.LogGroup
	logStream := v.logInfo.LogStream
	nextToken := v.nextToken
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parentCtx, apiTimeout)
		defer cancel()
		events, newToken, err := client.GetLogEvents(ctx, logGroup, logStream, nextToken, 100)
		if err != nil {
			// If the parent context was cancelled (view closed), silently drop
			if parentCtx.Err() != nil {
				return nil
			}
			return ErrorMsg{Err: err}
		}
		return logEventsMsg{events: events, nextToken: newToken}
	}
}

func (v *LogView) formatLogLine(event awsclient.LogEvent) string {
	ts := event.Timestamp.Local().Format("15:04:05")
	msg := strings.TrimRight(event.Message, "\n")
	upper := strings.ToUpper(msg)
	var style lipgloss.Style
	switch {
	case strings.Contains(upper, "ERROR"), strings.Contains(upper, "FATAL"):
		style = lipgloss.NewStyle().Foreground(colorRed)
	case strings.Contains(upper, "WARN"):
		style = lipgloss.NewStyle().Foreground(colorYellow)
	case strings.Contains(upper, "DEBUG"), strings.Contains(upper, "TRACE"):
		style = lipgloss.NewStyle().Foreground(colorOverlay1)
	default:
		style = lipgloss.NewStyle().Foreground(colorText)
	}
	return fmt.Sprintf("%s %s",
		lipgloss.NewStyle().Foreground(colorBlue).Render(ts),
		style.Render(msg))
}

func (v *LogView) addLogLine(event awsclient.LogEvent) {
	v.logLines = append(v.logLines, v.formatLogLine(event))
	if len(v.logLines) > maxLogLines {
		v.logLines = v.logLines[len(v.logLines)-maxLogLines:]
	}
}

func (v *LogView) computeSearchMatches() {
	v.searchMatches = nil
	if v.searchText == "" {
		return
	}
	lower := strings.ToLower(v.searchText)
	for i, line := range v.logLines {
		if strings.Contains(strings.ToLower(line), lower) {
			v.searchMatches = append(v.searchMatches, i)
		}
	}
	if len(v.searchMatches) > 0 {
		v.searchCurrent = len(v.searchMatches) - 1 // start at last match
	}
}

func (v *LogView) gotoCurrentMatch() {
	if len(v.searchMatches) == 0 || !v.ready {
		return
	}
	lineIdx := v.searchMatches[v.searchCurrent]
	// Approximate: set viewport to show the matching line
	v.autoScroll = false
	v.viewport.SetYOffset(lineIdx)
}

func (v *LogView) updateViewport() {
	if !v.ready {
		return
	}
	lines := make([]string, len(v.logLines))
	copy(lines, v.logLines)

	// Highlight search matches
	if v.searchText != "" {
		matchSet := make(map[int]bool)
		for _, idx := range v.searchMatches {
			matchSet[idx] = true
		}
		highlightStyle := lipgloss.NewStyle().Background(colorSurface2)
		for i, line := range lines {
			if matchSet[i] {
				lines[i] = highlightStyle.Render(line)
			}
		}
	}

	content := strings.Join(lines, "\n")
	v.viewport.SetContent(content)
	if v.autoScroll {
		v.viewport.GotoBottom()
	}
}

// Close implements the Closeable interface for cleanup on stack removal.
func (v *LogView) Close() {
	v.closed = true
	v.polling = false
	v.streaming = false
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
	if v.pollCancel != nil {
		v.pollCancel()
		v.pollCancel = nil
	}
}

func (v *LogView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		v.updateViewport()
		return v, nil

	case themeChangedMsg:
		v.updateViewport()
		return v, nil

	case logInfoMsg:
		v.logInfo = msg.info
		v.logLines = append(v.logLines, fmt.Sprintf("Log Group: %s", v.logInfo.LogGroup))
		v.logLines = append(v.logLines, fmt.Sprintf("Log Stream: %s", v.logInfo.LogStream))
		v.logLines = append(v.logLines, "---")
		v.updateViewport()

		// Try LiveTail first (runs asynchronously via tea.Cmd)
		if v.logInfo.LogGroupARN != "" {
			return v, v.startLiveTail()
		}
		// Fallback to polling — requires logStream
		if v.logInfo.LogStream != "" {
			v.logLines = append(v.logLines, "Streaming (polling fallback)...")
			v.updateViewport()
			return v, v.startPolling()
		}
		// No stream available
		v.logLines = append(v.logLines, "No log stream found. Check task definition log configuration.")
		v.updateViewport()
		return v, nil

	case liveTailStartedMsg:
		v.cancel = msg.cancel
		v.eventCh = msg.eventCh
		v.streaming = true
		v.logLines = append(v.logLines, "Streaming (LiveTail)...")
		v.updateViewport()
		return v, waitForLogEvent(v.eventCh)

	case liveTailFailedMsg:
		v.logLines = append(v.logLines, fmt.Sprintf("LiveTail unavailable: %v", msg.err))
		v.updateViewport()
		// Fallback to polling
		if v.logInfo != nil && v.logInfo.LogStream != "" {
			v.logLines = append(v.logLines, "Streaming (polling fallback)...")
			v.updateViewport()
			return v, v.startPolling()
		}
		return v, nil

	case logEventMsg:
		if v.closed {
			return v, nil
		}
		v.addLogLine(msg.event)
		v.updateViewport()
		if v.streaming && v.eventCh != nil {
			return v, waitForLogEvent(v.eventCh)
		}
		return v, nil

	case logStreamEndMsg:
		v.streaming = false
		if v.logInfo != nil && !v.polling && !v.closed {
			v.logLines = append(v.logLines, "--- LiveTail ended, switching to polling ---")
			v.updateViewport()
			return v, v.startPolling()
		}
		return v, nil

	case logEventsMsg:
		for _, e := range msg.events {
			v.addLogLine(e)
		}
		if msg.nextToken != "" {
			v.nextToken = msg.nextToken
		}
		v.updateViewport()
		return v, nil

	case logPollTickMsg:
		if v.polling && v.logInfo != nil && !v.closed && v.pollCtx != nil {
			return v, tea.Batch(v.pollLogsWithCtx(v.pollCtx), v.pollTickCmd())
		}
		return v, nil

	case tea.KeyMsg:
		// Search input mode
		if v.searching {
			switch msg.String() {
			case "enter":
				v.searching = false
				v.searchText = v.searchInput.Value()
				v.searchInput.Blur()
				v.computeSearchMatches()
				v.updateViewport()
				v.gotoCurrentMatch()
				return v, nil
			case "esc":
				v.searching = false
				v.searchInput.Blur()
				return v, nil
			}
			var cmd tea.Cmd
			v.searchInput, cmd = v.searchInput.Update(msg)
			return v, cmd
		}

		switch msg.String() {
		case "esc":
			if v.searchText != "" {
				v.searchText = ""
				v.searchInput.SetValue("")
				v.searchMatches = nil
				v.updateViewport()
				return v, nil
			}
			v.Close()
			return v, func() tea.Msg { return PopViewMsg{} }
		case "/":
			v.searching = true
			v.searchInput.Focus()
			return v, textinput.Blink
		case "n":
			if len(v.searchMatches) > 0 {
				v.searchCurrent = (v.searchCurrent + 1) % len(v.searchMatches)
				v.gotoCurrentMatch()
			}
			return v, nil
		case "N":
			if len(v.searchMatches) > 0 {
				v.searchCurrent = (v.searchCurrent - 1 + len(v.searchMatches)) % len(v.searchMatches)
				v.gotoCurrentMatch()
			}
			return v, nil
		case "f":
			v.autoScroll = !v.autoScroll
			if v.autoScroll {
				v.viewport.GotoBottom()
			}
			return v, nil
		case "G":
			v.autoScroll = true
			v.viewport.GotoBottom()
			return v, nil
		case "g":
			v.autoScroll = false
			v.viewport.GotoTop()
			return v, nil
		}
	}

	if v.ready {
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		if v.viewport.AtBottom() {
			v.autoScroll = true
		} else if v.autoScroll {
			switch msg.(type) {
			case tea.KeyMsg, tea.MouseMsg:
				v.autoScroll = false
			}
		}
		return v, cmd
	}
	return v, nil
}

func (v *LogView) View() string {
	if !v.ready {
		return loadingStyle.Render("  Loading logs...")
	}
	var sb strings.Builder
	if v.searching {
		sb.WriteString("  Search: ")
		sb.WriteString(v.searchInput.View())
		sb.WriteString("\n")
	} else if v.searchText != "" {
		info := fmt.Sprintf("  Search: %s (%d/%d matches, press Esc to clear)",
			v.searchText, v.searchCurrent+1, len(v.searchMatches))
		if len(v.searchMatches) == 0 {
			info = fmt.Sprintf("  Search: %s (no matches, press Esc to clear)", v.searchText)
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(info))
		sb.WriteString("\n")
	}
	sb.WriteString(v.viewport.View())
	return sb.String()
}
