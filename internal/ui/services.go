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

type serviceTickMsg time.Time

func NewServiceView(client *awsclient.Client, cluster string) *ServiceView {
	ti := textinput.New()
	ti.Placeholder = "Filter services..."
	ti.CharLimit = 50

	return &ServiceView{
		client:      client,
		cluster:     cluster,
		taskDefs:    make(map[string]*awsclient.TaskDefinitionInfo),
		metrics:     make(map[string]*awsclient.ServiceMetrics),
		filterInput: ti,
	}
}

func (v *ServiceView) Title() string { return "Services" }

func (v *ServiceView) ShortcutHelp() []Shortcut {
	if v.filtering {
		return []Shortcut{
			{Key: "Enter", Desc: "Apply"},
			{Key: "Esc", Desc: "Cancel"},
		}
	}
	return []Shortcut{
		{Key: "Enter", Desc: "Tasks"},
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
	return func() tea.Msg {
		services, err := v.client.ListServices(context.Background(), v.cluster)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return servicesLoadedMsg{services: services}
	}
}

func (v *ServiceView) fetchTaskDefs() tea.Cmd {
	return func() tea.Msg {
		defs := make(map[string]*awsclient.TaskDefinitionInfo)
		seen := make(map[string]bool)
		for _, svc := range v.services {
			if svc.TaskDef == "" || seen[svc.TaskDef] {
				continue
			}
			seen[svc.TaskDef] = true
			td, err := v.client.DescribeTaskDefinition(context.Background(), svc.TaskDef)
			if err != nil {
				continue
			}
			defs[svc.TaskDef] = td
		}
		return taskDefsLoadedMsg{defs: defs}
	}
}

func (v *ServiceView) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		var names []string
		for _, svc := range v.services {
			names = append(names, svc.Name)
		}
		metrics, err := v.client.GetServiceMetrics(context.Background(), v.cluster, names)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return serviceMetricsLoadedMsg{metrics: metrics}
	}
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

	case serviceTickMsg:
		if v.loaded {
			return v, tea.Batch(v.fetchServices(), v.tickCmd())
		}
		return v, v.tickCmd()

	case tea.KeyMsg:
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

		switch msg.String() {
		case "enter":
			if len(v.table.Rows()) == 0 {
				return v, nil
			}
			row := v.table.SelectedRow()
			if len(row) > 0 {
				serviceName := row[0]
				taskView := NewTaskView(v.client, v.cluster, serviceName)
				return v, func() tea.Msg {
					return PushViewMsg{View: taskView}
				}
			}
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
	return sb.String()
}

func (v *ServiceView) rebuildTable() {
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
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	v.table = t
}
