package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

const maxLogLines = 10000

type LogView struct {
	client     *awsclient.Client
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

	// LiveTail
	eventCh chan awsclient.LogEvent
	cancel  context.CancelFunc

	// Polling fallback
	nextToken string
	polling   bool
}

type logInfoMsg struct {
	info *awsclient.LogInfo
}

type logEventMsg struct {
	event awsclient.LogEvent
}

type logStreamEndMsg struct{}

type logPollTickMsg time.Time

type logEventsMsg struct {
	events    []awsclient.LogEvent
	nextToken string
}

func NewLogView(client *awsclient.Client, cluster string, task *awsclient.TaskInfo) *LogView {
	return &LogView{
		client:     client,
		cluster:    cluster,
		task:       task,
		autoScroll: true,
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
	return []Shortcut{
		{Key: "f", Desc: "Follow"},
		{Key: "G", Desc: "Bottom"},
		{Key: "g", Desc: "Top"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *LogView) Init() tea.Cmd {
	return v.fetchLogInfo()
}

func (v *LogView) fetchLogInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := v.client.GetLogInfo(
			context.Background(),
			v.task.TaskDefARN,
			v.task.ContainerName,
			v.task.TaskID,
		)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return logInfoMsg{info: info}
	}
}

func (v *LogView) startLiveTail() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	v.cancel = cancel
	v.eventCh = make(chan awsclient.LogEvent, 100)

	streamNames := []string{}
	if v.logInfo.LogStream != "" {
		streamNames = []string{v.logInfo.LogStream}
	}

	err := v.client.StartLiveTail(ctx, v.logInfo.LogGroupARN, streamNames, "", v.eventCh)
	if err != nil {
		cancel()
		v.cancel = nil
		v.eventCh = nil
		v.logLines = append(v.logLines, fmt.Sprintf("LiveTail unavailable: %v", err))
		return nil
	}
	v.streaming = true
	return waitForLogEvent(v.eventCh)
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
	return tea.Batch(v.pollLogs(), v.pollTickCmd())
}

func (v *LogView) pollTickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return logPollTickMsg(t)
	})
}

func (v *LogView) pollLogs() tea.Cmd {
	return func() tea.Msg {
		events, nextToken, err := v.client.GetLogEvents(
			context.Background(),
			v.logInfo.LogGroup,
			v.logInfo.LogStream,
			v.nextToken,
			100,
		)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return logEventsMsg{events: events, nextToken: nextToken}
	}
}

func (v *LogView) addLogLine(event awsclient.LogEvent) {
	line := fmt.Sprintf("%s %s",
		event.Timestamp.Local().Format("15:04:05"),
		strings.TrimRight(event.Message, "\n"),
	)
	v.logLines = append(v.logLines, line)
	if len(v.logLines) > maxLogLines {
		v.logLines = v.logLines[len(v.logLines)-maxLogLines:]
	}
}

func (v *LogView) updateViewport() {
	if !v.ready {
		return
	}
	content := strings.Join(v.logLines, "\n")
	v.viewport.SetContent(content)
	if v.autoScroll {
		v.viewport.GotoBottom()
	}
}

// Close implements the Closeable interface for cleanup on stack removal.
func (v *LogView) Close() {
	v.polling = false
	v.streaming = false
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
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

	case logInfoMsg:
		v.logInfo = msg.info
		v.logLines = append(v.logLines, fmt.Sprintf("Log Group: %s", v.logInfo.LogGroup))
		v.logLines = append(v.logLines, fmt.Sprintf("Log Stream: %s", v.logInfo.LogStream))
		v.logLines = append(v.logLines, "---")
		v.updateViewport()

		// Try LiveTail first (works even without specific stream)
		if v.logInfo.LogGroupARN != "" {
			cmd := v.startLiveTail()
			if cmd != nil {
				v.logLines = append(v.logLines, "Streaming (LiveTail)...")
				v.updateViewport()
				return v, cmd
			}
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

	case logEventMsg:
		v.addLogLine(msg.event)
		v.updateViewport()
		if v.streaming && v.eventCh != nil {
			return v, waitForLogEvent(v.eventCh)
		}
		return v, nil

	case logStreamEndMsg:
		v.streaming = false
		if v.logInfo != nil && !v.polling {
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
		if v.polling && v.logInfo != nil {
			return v, tea.Batch(v.pollLogs(), v.pollTickCmd())
		}
		return v, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			v.Close()
			return v, func() tea.Msg { return PopViewMsg{} }
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
	return v.viewport.View()
}
