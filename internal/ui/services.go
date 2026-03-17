package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type ServiceView struct {
	client      *awsclient.Client
	cluster     string
	table       table.Model
	services    []awsclient.ServiceInfo
	taskDefs    map[string]*awsclient.TaskDefinitionInfo
	metrics     map[string]*awsclient.ServiceMetrics
	width       int
	height      int
	loaded      bool
	filterInput textinput.Model
	filtering   bool
	filterText  string
	// Confirm action state
	confirmAction  string // "force-deploy" | "update-count" | ""
	confirmMsg     string
	pendingService string
	pendingCount   int32
	// Count input state
	inputtingCount bool
	countInput     textinput.Model
}

type servicesLoadedMsg struct {
	services []awsclient.ServiceInfo
}

type taskDefsLoadedMsg struct {
	defs map[string]*awsclient.TaskDefinitionInfo
}

type serviceMetricsLoadedMsg struct {
	metrics map[string]*awsclient.ServiceMetrics
}

type serviceActionDoneMsg struct {
	message string
}

type serviceTickMsg time.Time

func NewServiceView(client *awsclient.Client, cluster string) *ServiceView {
	ti := textinput.New()
	ti.Placeholder = "Filter services..."
	ti.CharLimit = 50

	ci := textinput.New()
	ci.Placeholder = "Enter desired count..."
	ci.CharLimit = 5

	return &ServiceView{
		client:      client,
		cluster:     cluster,
		taskDefs:    make(map[string]*awsclient.TaskDefinitionInfo),
		metrics:     make(map[string]*awsclient.ServiceMetrics),
		filterInput: ti,
		countInput:  ci,
	}
}

func (v *ServiceView) Title() string { return "Services" }

func (v *ServiceView) ShortcutHelp() []Shortcut {
	if v.confirmAction != "" {
		return []Shortcut{
			{Key: "y", Desc: "Confirm"},
			{Key: "n/Esc", Desc: "Cancel"},
		}
	}
	if v.inputtingCount {
		return []Shortcut{
			{Key: "Enter", Desc: "Submit"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	if v.filtering {
		return []Shortcut{
			{Key: "Enter", Desc: "Apply"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	return []Shortcut{
		{Key: "Enter", Desc: "Tasks"},
		{Key: "e", Desc: "Events"},
		{Key: "f", Desc: "Force Deploy"},
		{Key: "d", Desc: "Desired Count"},
		{Key: "/", Desc: "Filter"},
		{Key: "r", Desc: "Refresh"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *ServiceView) Init() tea.Cmd {
	return tea.Batch(v.fetchServices(), v.tickCmd())
}

func (v *ServiceView) tickCmd() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return serviceTickMsg(t)
	})
}

func (v *ServiceView) fetchServices() tea.Cmd {
	client := v.client
	cluster := v.cluster
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		services, err := client.ListServices(ctx, cluster)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return servicesLoadedMsg{services: services}
	}
}

func (v *ServiceView) fetchTaskDefs() tea.Cmd {
	// Copy services slice to avoid data race with concurrent updates
	services := make([]awsclient.ServiceInfo, len(v.services))
	copy(services, v.services)
	client := v.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		defs := make(map[string]*awsclient.TaskDefinitionInfo)
		seen := make(map[string]bool)
		for _, svc := range services {
			if svc.TaskDef == "" || seen[svc.TaskDef] {
				continue
			}
			seen[svc.TaskDef] = true
			td, err := client.DescribeTaskDefinition(ctx, svc.TaskDef)
			if err != nil {
				continue
			}
			defs[svc.TaskDef] = td
		}
		return taskDefsLoadedMsg{defs: defs}
	}
}

func (v *ServiceView) fetchMetrics() tea.Cmd {
	// Copy data to avoid data race with concurrent updates
	names := make([]string, 0, len(v.services))
	for _, svc := range v.services {
		names = append(names, svc.Name)
	}
	client := v.client
	cluster := v.cluster
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		metrics, err := client.GetServiceMetrics(ctx, cluster, names)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return serviceMetricsLoadedMsg{metrics: metrics}
	}
}

func (v *ServiceView) selectedService() *awsclient.ServiceInfo {
	if len(v.table.Rows()) == 0 {
		return nil
	}
	row := v.table.SelectedRow()
	if len(row) == 0 {
		return nil
	}
	name := row[0]
	for i := range v.services {
		if v.services[i].Name == name {
			return &v.services[i]
		}
	}
	return nil
}

func (v *ServiceView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.loaded {
			v.rebuildTable()
		}
		return v, nil

	case servicesLoadedMsg:
		v.services = msg.services
		v.loaded = true
		v.rebuildTable()
		return v, tea.Batch(v.fetchTaskDefs(), v.fetchMetrics())

	case taskDefsLoadedMsg:
		v.taskDefs = msg.defs
		v.rebuildTable()
		return v, nil

	case serviceMetricsLoadedMsg:
		v.metrics = msg.metrics
		v.rebuildTable()
		return v, nil

	case themeChangedMsg:
		if v.loaded {
			v.rebuildTable()
		}
		return v, nil

	case serviceActionDoneMsg:
		v.confirmAction = ""
		v.confirmMsg = ""
		return v, tea.Batch(
			v.fetchServices(),
			func() tea.Msg { return StatusMsg{Message: msg.message} },
		)

	case serviceTickMsg:
		if v.loaded {
			return v, tea.Batch(v.fetchServices(), v.tickCmd())
		}
		return v, v.tickCmd()

	case tea.KeyMsg:
		// Priority 1: Confirm action mode
		if v.confirmAction != "" {
			switch msg.String() {
			case "y", "Y":
				action := v.confirmAction
				svcName := v.pendingService
				count := v.pendingCount
				client := v.client
				cluster := v.cluster
				v.confirmAction = ""
				v.confirmMsg = ""
				switch action {
				case "force-deploy":
					return v, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						err := client.ForceNewDeployment(ctx, cluster, svcName)
						if err != nil {
							return ErrorMsg{Err: err}
						}
						return serviceActionDoneMsg{message: fmt.Sprintf("Force deploy triggered: %s", svcName)}
					}
				case "update-count":
					return v, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						err := client.UpdateDesiredCount(ctx, cluster, svcName, count)
						if err != nil {
							return ErrorMsg{Err: err}
						}
						return serviceActionDoneMsg{message: fmt.Sprintf("Desired count updated to %d: %s", count, svcName)}
					}
				}
			case "n", "N", "esc":
				v.confirmAction = ""
				v.confirmMsg = ""
			}
			return v, nil
		}

		// Priority 2: Count input mode
		if v.inputtingCount {
			switch msg.String() {
			case "enter":
				val := v.countInput.Value()
				count, err := strconv.Atoi(val)
				if err != nil || count < 0 || count > 10000 {
					v.inputtingCount = false
					v.countInput.Blur()
					v.countInput.SetValue("")
					return v, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("invalid count: %s (must be 0-10000)", val)}
					}
				}
				v.inputtingCount = false
				v.countInput.Blur()
				v.countInput.SetValue("")
				v.pendingCount = int32(count) //nolint:gosec // bounds checked above
				v.confirmAction = "update-count"
				v.confirmMsg = fmt.Sprintf("Update desired count of '%s' to %d? (y/n)", v.pendingService, count)
				return v, nil
			case "esc":
				v.inputtingCount = false
				v.countInput.Blur()
				v.countInput.SetValue("")
				return v, nil
			}
			var cmd tea.Cmd
			v.countInput, cmd = v.countInput.Update(msg)
			return v, cmd
		}

		// Priority 3: Filter mode
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

		// Priority 4: Normal mode
		switch msg.String() {
		case "enter":
			svc := v.selectedService()
			if svc != nil {
				taskView := NewTaskView(v.client, v.cluster, svc.Name)
				return v, func() tea.Msg {
					return PushViewMsg{View: taskView}
				}
			}
		case "e":
			svc := v.selectedService()
			if svc != nil {
				eventsView := NewServiceEventsView(v.client, v.cluster, svc.Name)
				return v, func() tea.Msg {
					return PushViewMsg{View: eventsView}
				}
			}
		case "f":
			svc := v.selectedService()
			if svc != nil {
				v.pendingService = svc.Name
				v.confirmAction = "force-deploy"
				v.confirmMsg = fmt.Sprintf("Force new deployment for '%s'? (y/n)", svc.Name)
			}
			return v, nil
		case "d":
			svc := v.selectedService()
			if svc != nil {
				v.pendingService = svc.Name
				v.inputtingCount = true
				v.countInput.SetValue(fmt.Sprintf("%d", svc.DesiredCount))
				v.countInput.Focus()
				return v, textinput.Blink
			}
			return v, nil
		case "/":
			v.filtering = true
			v.filterInput.Focus()
			return v, textinput.Blink
		case "r":
			v.loaded = false
			return v, v.fetchServices()
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

func (v *ServiceView) View() string {
	if !v.loaded {
		return loadingStyle.Render("  Loading services...")
	}

	var sb strings.Builder

	// Filter bar (inline — not modal)
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

	// Modal overlays for confirm / count input
	if v.confirmAction != "" {
		titleStyle := lipgloss.NewStyle().Foreground(colorPeach).Bold(true)
		hintStyle := lipgloss.NewStyle().Foreground(colorSubtext0)
		content := titleStyle.Render(v.confirmMsg) + "\n\n" +
			hintStyle.Render("  <y> Confirm    <n/Esc> Cancel")
		box := OverlayBoxStyle().Render(content)
		return RenderOverlay(base, box, v.width, v.height)
	}
	if v.inputtingCount {
		titleStyle := lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
		content := titleStyle.Render("Desired Count") + "\n\n" +
			"  " + v.countInput.View() + "\n\n" +
			lipgloss.NewStyle().Foreground(colorSubtext0).Render("  <Enter> Submit    <Esc> Cancel")
		box := OverlayBoxStyle().Render(content)
		return RenderOverlay(base, box, v.width, v.height)
	}

	return base
}

func (v *ServiceView) rebuildTable() {
	// Preserve cursor position across rebuilds
	prevCursor := v.table.Cursor()

	rcols := []responsiveColumn{
		{Title: "Service", MinWidth: 15, Flex: 3},
		{Title: "Status", MinWidth: 10, Flex: 0},
		{Title: "Run/Des", MinWidth: 9, Flex: 0},
		{Title: "CPU Res", MinWidth: 9, Flex: 0},
		{Title: "Mem Res", MinWidth: 9, Flex: 0},
		{Title: "CPU %", MinWidth: 8, Flex: 0},
		{Title: "Mem %", MinWidth: 8, Flex: 0},
		{Title: "Last Event", MinWidth: 15, Flex: 4},
	}
	widths := calcColumnWidths(rcols, v.width)
	columns := make([]table.Column, len(rcols))
	for i, rc := range rcols {
		columns[i] = table.Column{Title: rc.Title, Width: widths[i]}
	}

	var rows []table.Row
	for _, svc := range v.services {
		if v.filterText != "" && !strings.Contains(
			strings.ToLower(svc.Name), strings.ToLower(v.filterText)) {
			continue
		}

		cpuRes, memRes := "...", "..."
		if td, ok := v.taskDefs[svc.TaskDef]; ok {
			cpuRes = td.CPU
			memRes = td.Memory
			if cpuRes == "" {
				cpuRes = "-"
			}
			if memRes == "" {
				memRes = "-"
			}
		}

		cpuPct, memPct := "...", "..."
		if m, ok := v.metrics[svc.Name]; ok {
			if m.CPUUtilization != nil {
				cpuPct = fmt.Sprintf("%.1f%%", *m.CPUUtilization)
			} else {
				cpuPct = "-"
			}
			if m.MemoryUtilization != nil {
				memPct = fmt.Sprintf("%.1f%%", *m.MemoryUtilization)
			} else {
				memPct = "-"
			}
		}

		runDes := fmt.Sprintf("%d/%d", svc.RunningCount, svc.DesiredCount)

		rows = append(rows, table.Row{
			svc.Name,
			svc.Status,
			runDes,
			cpuRes,
			memRes,
			cpuPct,
			memPct,
			svc.LastEvent,
		})
	}

	tableHeight := v.height - 2
	if v.filtering || v.filterText != "" {
		tableHeight -= 2
	}
	if tableHeight < 5 {
		tableHeight = 5
	}

	t := table.New(
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
	t.SetStyles(s)

	// Restore cursor, clamped to row count
	if prevCursor >= len(rows) {
		prevCursor = len(rows) - 1
	}
	if prevCursor < 0 {
		prevCursor = 0
	}
	t.SetCursor(prevCursor)

	v.table = t
}
