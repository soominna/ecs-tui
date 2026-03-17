package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
	execpkg "github.com/soominna/ecs-tui/internal/exec"
)

type TaskView struct {
	client      *awsclient.Client
	cluster     string
	service     string
	table       table.Model
	tasks       []awsclient.TaskInfo
	width       int
	height      int
	loaded      bool
	filterInput textinput.Model
	filtering   bool
	filterText  string
	// Confirm action state
	confirmAction  string // "stop-task" | ""
	confirmMsg     string
	pendingTaskARN string
}

type tasksLoadedMsg struct {
	tasks []awsclient.TaskInfo
}

type taskActionDoneMsg struct {
	message string
}

type taskTickMsg time.Time

func NewTaskView(client *awsclient.Client, cluster, service string) *TaskView {
	ti := textinput.New()
	ti.Placeholder = "Filter tasks..."
	ti.CharLimit = 50

	return &TaskView{
		client:      client,
		cluster:     cluster,
		service:     service,
		filterInput: ti,
	}
}

func (v *TaskView) Title() string { return fmt.Sprintf("Tasks (%s)", v.service) }

func (v *TaskView) ShortcutHelp() []Shortcut {
	if v.confirmAction != "" {
		return []Shortcut{
			{Key: "y", Desc: "Confirm"},
			{Key: "n/Esc", Desc: "Cancel"},
		}
	}
	if v.filtering {
		return []Shortcut{
			{Key: "Enter", Desc: "Apply"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	return []Shortcut{
		{Key: "Enter/d", Desc: "Detail"},
		{Key: "l", Desc: "Logs"},
		{Key: "e", Desc: "Exec"},
		{Key: "s", Desc: "Stop Task"},
		{Key: "/", Desc: "Filter"},
		{Key: "r", Desc: "Refresh"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *TaskView) Init() tea.Cmd {
	return tea.Batch(v.fetchTasks(), v.tickCmd())
}

func (v *TaskView) tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return taskTickMsg(t)
	})
}

func (v *TaskView) fetchTasks() tea.Cmd {
	client := v.client
	cluster := v.cluster
	service := v.service
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		tasks, err := client.ListTasks(ctx, cluster, service)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

func (v *TaskView) selectedTask() *awsclient.TaskInfo {
	if len(v.table.Rows()) == 0 {
		return nil
	}
	row := v.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}
	taskID := row[0]
	for i := range v.tasks {
		if v.tasks[i].TaskID == taskID {
			return &v.tasks[i]
		}
	}
	return nil
}

func (v *TaskView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.loaded {
			v.rebuildTable()
		}
		return v, nil

	case tasksLoadedMsg:
		v.tasks = msg.tasks
		v.loaded = true
		v.rebuildTable()
		return v, nil

	case themeChangedMsg:
		if v.loaded {
			v.rebuildTable()
		}
		return v, nil

	case taskActionDoneMsg:
		v.confirmAction = ""
		v.confirmMsg = ""
		return v, tea.Batch(
			v.fetchTasks(),
			func() tea.Msg { return StatusMsg{Message: msg.message} },
		)

	case taskTickMsg:
		if v.loaded {
			return v, tea.Batch(v.fetchTasks(), v.tickCmd())
		}
		return v, v.tickCmd()

	case tea.KeyMsg:
		// Priority 1: Confirm action mode
		if v.confirmAction != "" {
			switch msg.String() {
			case "y", "Y":
				action := v.confirmAction
				taskARN := v.pendingTaskARN
				client := v.client
				cluster := v.cluster
				v.confirmAction = ""
				v.confirmMsg = ""
				if action == "stop-task" {
					return v, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
						defer cancel()
						err := client.StopTask(ctx, cluster, taskARN, "Stopped via ECS-TUI")
						if err != nil {
							return ErrorMsg{Err: err}
						}
						return taskActionDoneMsg{message: "Task stop requested"}
					}
				}
			case "n", "N", "esc":
				v.confirmAction = ""
				v.confirmMsg = ""
			}
			return v, nil
		}

		// Priority 2: Filter mode
		if v.filtering {
			switch msg.String() {
			case "enter":
				v.filtering = false
				v.filterText = v.filterInput.Value()
				v.filterInput.Blur()
				v.rebuildTable()
				return v, nil
			case "esc":
				v.filtering = false
				v.filterInput.SetValue("")
				v.filterInput.Blur()
				v.filterText = ""
				v.rebuildTable()
				return v, nil
			}
			var cmd tea.Cmd
			v.filterInput, cmd = v.filterInput.Update(msg)
			return v, cmd
		}

		// Priority 3: Normal mode
		switch msg.String() {
		case "enter", "d":
			task := v.selectedTask()
			if task != nil {
				detailView := NewDetailView(v.client, task)
				return v, func() tea.Msg {
					return PushViewMsg{View: detailView}
				}
			}
		case "l":
			task := v.selectedTask()
			if task == nil {
				return v, nil
			}
			if task.TaskDefARN == "" {
				return v, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("no task definition found for this task")}
				}
			}
			logView := NewLogView(v.client, v.cluster, task)
			return v, func() tea.Msg {
				return PushViewMsg{View: logView}
			}
		case "e":
			task := v.selectedTask()
			if task == nil {
				return v, nil
			}
			if task.Status != "RUNNING" {
				return v, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("exec requires RUNNING task (current: %s)", task.Status)}
				}
			}
			return v, execpkg.ExecContainer(
				v.client.Profile,
				v.client.Region,
				v.cluster,
				v.service,
				task.TaskID,
				task.ContainerName,
			)
		case "s":
			task := v.selectedTask()
			if task != nil {
				v.pendingTaskARN = task.TaskARN
				v.confirmAction = "stop-task"
				v.confirmMsg = fmt.Sprintf("Stop task '%s'? (y/n)", task.TaskID)
			}
			return v, nil
		case "/":
			v.filtering = true
			v.filterInput.Focus()
			return v, textinput.Blink
		case "r":
			v.loaded = false
			return v, v.fetchTasks()
		case "esc":
			if v.filterText != "" {
				v.filterText = ""
				v.filterInput.SetValue("")
				v.rebuildTable()
				return v, nil
			}
			return v, func() tea.Msg { return PopViewMsg{} }
		}
	}

	if v.loaded {
		var cmd tea.Cmd
		v.table, cmd = v.table.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *TaskView) View() string {
	if !v.loaded {
		return loadingStyle.Render("  Loading tasks...")
	}

	var sb strings.Builder

	// Filter bar (inline)
	if v.filtering {
		sb.WriteString("  Filter: ")
		sb.WriteString(v.filterInput.View())
		sb.WriteString("\n")
	} else if v.filterText != "" {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(
			fmt.Sprintf("  Filter: %s (press Esc to clear)", v.filterText)))
		sb.WriteString("\n")
	}

	sb.WriteString(v.table.View())
	base := sb.String()

	// Modal overlay for confirm
	if v.confirmAction != "" {
		titleStyle := lipgloss.NewStyle().Foreground(colorPeach).Bold(true)
		hintStyle := lipgloss.NewStyle().Foreground(colorSubtext0)
		content := titleStyle.Render(v.confirmMsg) + "\n\n" +
			hintStyle.Render("  <y> Confirm    <n/Esc> Cancel")
		box := OverlayBoxStyle().Render(content)
		return RenderOverlay(base, box, v.width, v.height)
	}

	return base
}

func (v *TaskView) rebuildTable() {
	// Preserve cursor position across rebuilds
	prevCursor := v.table.Cursor()

	rcols := []responsiveColumn{
		{Title: "Task ID", MinWidth: 12, Flex: 3},
		{Title: "Status", MinWidth: 10, Flex: 0},
		{Title: "IP", MinWidth: 15, Flex: 0},
		{Title: "Started", MinWidth: 19, Flex: 1},
		{Title: "Health", MinWidth: 10, Flex: 0},
		{Title: "Container", MinWidth: 12, Flex: 2},
	}
	widths := calcColumnWidths(rcols, v.width)
	columns := make([]table.Column, len(rcols))
	for i, rc := range rcols {
		columns[i] = table.Column{Title: rc.Title, Width: widths[i]}
	}

	var rows []table.Row
	for _, t := range v.tasks {
		if v.filterText != "" && !strings.Contains(
			strings.ToLower(t.TaskID), strings.ToLower(v.filterText)) &&
			!strings.Contains(strings.ToLower(t.ContainerName), strings.ToLower(v.filterText)) {
			continue
		}

		started := "-"
		if t.StartedAt != nil {
			started = t.StartedAt.Local().Format("2006-01-02 15:04:05")
		}

		health := t.HealthStatus
		if health == "" || health == "UNKNOWN" {
			health = "-"
		}

		rows = append(rows, table.Row{
			t.TaskID,
			t.Status,
			t.IP,
			started,
			health,
			t.ContainerName,
		})
	}

	tableHeight := v.height - 2
	if v.filtering || v.filterText != "" {
		tableHeight -= 2
	}
	if tableHeight < 5 {
		tableHeight = 5
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
	)

	s := table.DefaultStyles()
	hdr, sel := TableStyles()
	s.Header = hdr
	s.Selected = sel
	s.Cell = s.Cell.Foreground(colorText)
	tbl.SetStyles(s)

	// Restore cursor, clamped to row count
	if prevCursor >= len(rows) {
		prevCursor = len(rows) - 1
	}
	if prevCursor < 0 {
		prevCursor = 0
	}
	tbl.SetCursor(prevCursor)

	v.table = tbl
}
