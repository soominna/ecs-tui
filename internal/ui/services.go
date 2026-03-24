package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sync/errgroup"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type ServiceView struct {
	client      awsclient.ECSAPI
	cluster     string
	profile     string
	region      string
	table       table.Model
	services    []awsclient.ServiceInfo
	taskDefs    map[string]*awsclient.TaskDefinitionInfo
	metrics     map[string]*awsclient.ServiceMetrics
	deployments map[string]*awsclient.ServiceDeploymentInfo
	metricsHist map[string]*awsclient.ServiceMetricsHistory
	width       int
	height      int
	loaded      bool
	lastUpdated time.Time
	filterInput textinput.Model
	filtering   bool
	filterText  string
	// Confirm action state
	confirmAction  string // "force-deploy" | "update-count" | "enable-metrics" | ""
	confirmMsg     string
	pendingService string
	pendingCount   int32
	// Count input state
	inputtingCount bool
	countInput     textinput.Model
	// Read-only mode
	readOnly bool
	// Config
	refreshInterval time.Duration
	shell           string
	metricsEnabled  bool
}

type servicesLoadedMsg struct {
	services []awsclient.ServiceInfo
}

type taskDefsLoadedMsg struct {
	defs   map[string]*awsclient.TaskDefinitionInfo
	errors []string
}

type serviceMetricsLoadedMsg struct {
	metrics  map[string]*awsclient.ServiceMetrics
	fetchErr error // non-nil if API call failed (metrics set to empty so display shows "-")
}

type serviceDeploymentsLoadedMsg struct {
	deployments map[string]*awsclient.ServiceDeploymentInfo
}

type serviceMetricsHistoryLoadedMsg struct {
	history map[string]*awsclient.ServiceMetricsHistory
}

type serviceActionDoneMsg struct {
	message string
}

type serviceTickMsg time.Time

func NewServiceView(client awsclient.ECSAPI, cluster, profile, region string, readOnly bool, refreshInterval time.Duration, shell string, metricsEnabled bool, taskDefCache map[string]*awsclient.TaskDefinitionInfo) *ServiceView {
	ti := textinput.New()
	ti.Placeholder = "Filter services..."
	ti.CharLimit = 50

	ci := textinput.New()
	ci.Placeholder = "Enter desired count..."
	ci.CharLimit = 5

	// Initialize taskDefs from app-level cache
	defs := make(map[string]*awsclient.TaskDefinitionInfo)
	for k, v := range taskDefCache {
		defs[k] = v
	}

	return &ServiceView{
		client:          client,
		cluster:         cluster,
		profile:         profile,
		region:          region,
		taskDefs:        defs,
		metrics:         make(map[string]*awsclient.ServiceMetrics),
		deployments:     make(map[string]*awsclient.ServiceDeploymentInfo),
		metricsHist:     make(map[string]*awsclient.ServiceMetricsHistory),
		filterInput:     ti,
		countInput:      ci,
		readOnly:        readOnly,
		refreshInterval: refreshInterval,
		shell:           shell,
		metricsEnabled:  metricsEnabled,
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
	shortcuts := []Shortcut{
		{Key: "Enter", Desc: "Tasks"},
		{Key: "e", Desc: "Events"},
		{Key: "D", Desc: "Deploys"},
	}
	if !v.readOnly {
		shortcuts = append(shortcuts,
			Shortcut{Key: "f", Desc: "Force Deploy"},
			Shortcut{Key: "d", Desc: "Desired Count"},
		)
	}
	if v.metricsEnabled {
		shortcuts = append(shortcuts, Shortcut{Key: "m", Desc: "Metrics Off"})
	} else {
		shortcuts = append(shortcuts, Shortcut{Key: "m", Desc: "Metrics On"})
	}
	shortcuts = append(shortcuts,
		Shortcut{Key: "/", Desc: "Filter"},
		Shortcut{Key: "r", Desc: "Refresh"},
		Shortcut{Key: "Esc", Desc: "Back"},
	)
	return shortcuts
}

func (v *ServiceView) Init() tea.Cmd {
	return tea.Batch(v.fetchServices(), v.tickCmd())
}

func (v *ServiceView) tickCmd() tea.Cmd {
	return newTickCmd(v.refreshInterval, func(t time.Time) serviceTickMsg { return serviceTickMsg(t) })
}

func (v *ServiceView) fetchServices() tea.Cmd {
	client := v.client
	cluster := v.cluster
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
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
	// Copy existing cache keys to skip already-fetched defs
	cached := make(map[string]bool)
	for k := range v.taskDefs {
		cached[k] = true
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()

		// Collect unique task definitions to fetch (skip cached)
		var unique []string
		seen := make(map[string]bool)
		for _, svc := range services {
			if svc.TaskDef == "" || seen[svc.TaskDef] || cached[svc.TaskDef] {
				continue
			}
			seen[svc.TaskDef] = true
			unique = append(unique, svc.TaskDef)
		}

		// Fetch in parallel with bounded concurrency
		var mu sync.Mutex
		defs := make(map[string]*awsclient.TaskDefinitionInfo)
		var fetchErrors []string

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(5)
		for _, taskDef := range unique {
			td := taskDef
			g.Go(func() error {
				info, err := client.DescribeTaskDefinition(gctx, td)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					fetchErrors = append(fetchErrors, fmt.Sprintf("%s: %v", td, err))
					return nil // non-fatal
				}
				defs[td] = info
				return nil
			})
		}
		g.Wait()
		return taskDefsLoadedMsg{defs: defs, errors: fetchErrors}
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
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		metrics, err := client.GetServiceMetrics(ctx, cluster, names)
		if err != nil {
			// Return empty metrics so display shows "-" instead of staying at "..."
			empty := make(map[string]*awsclient.ServiceMetrics)
			for _, n := range names {
				empty[n] = &awsclient.ServiceMetrics{}
			}
			return serviceMetricsLoadedMsg{metrics: empty, fetchErr: err}
		}
		return serviceMetricsLoadedMsg{metrics: metrics}
	}
}

func (v *ServiceView) fetchDeployments() tea.Cmd {
	names := make([]string, 0, len(v.services))
	for _, svc := range v.services {
		names = append(names, svc.Name)
	}
	client := v.client
	cluster := v.cluster
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		result := make(map[string]*awsclient.ServiceDeploymentInfo)
		var mu sync.Mutex

		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(5)
		for _, name := range names {
			n := name
			g.Go(func() error {
				info, err := client.GetServiceDeployments(gctx, cluster, n)
				if err != nil {
					return nil // non-fatal
				}
				mu.Lock()
				result[n] = info
				mu.Unlock()
				return nil
			})
		}
		g.Wait()
		return serviceDeploymentsLoadedMsg{deployments: result}
	}
}

func (v *ServiceView) fetchMetricsHistory() tea.Cmd {
	names := make([]string, 0, len(v.services))
	for _, svc := range v.services {
		names = append(names, svc.Name)
	}
	client := v.client
	cluster := v.cluster
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
		defer cancel()
		history, err := client.GetServiceMetricsHistory(ctx, cluster, names, 8)
		if err != nil {
			// Silently return empty history — sparklines just won't show
			return serviceMetricsHistoryLoadedMsg{history: make(map[string]*awsclient.ServiceMetricsHistory)}
		}
		return serviceMetricsHistoryLoadedMsg{history: history}
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
		v.lastUpdated = time.Now()
		v.rebuildTable()
		cmds := []tea.Cmd{v.fetchTaskDefs(), v.fetchDeployments()}
		if v.metricsEnabled {
			cmds = append(cmds, v.fetchMetrics(), v.fetchMetricsHistory())
		}
		return v, tea.Batch(cmds...)

	case taskDefsLoadedMsg:
		for k, def := range msg.defs {
			v.taskDefs[k] = def
		}
		v.rebuildTable()
		var cmds []tea.Cmd
		if len(msg.defs) > 0 {
			// Update app-level cache
			cmds = append(cmds, func() tea.Msg {
				return taskDefCacheUpdateMsg{Defs: msg.defs}
			})
		}
		if len(msg.errors) > 0 {
			cmds = append(cmds, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("failed to fetch %d task definition(s)", len(msg.errors))}
			})
		}
		if len(cmds) > 0 {
			return v, tea.Batch(cmds...)
		}
		return v, nil

	case serviceMetricsLoadedMsg:
		v.metrics = msg.metrics
		v.rebuildTable()
		if msg.fetchErr != nil {
			return v, func() tea.Msg { return ErrorMsg{Err: msg.fetchErr} }
		}
		return v, nil

	case serviceDeploymentsLoadedMsg:
		v.deployments = msg.deployments
		v.rebuildTable()
		return v, nil

	case serviceMetricsHistoryLoadedMsg:
		v.metricsHist = msg.history
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
						ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
						defer cancel()
						err := client.ForceNewDeployment(ctx, cluster, svcName)
						if err != nil {
							return ErrorMsg{Err: err}
						}
						return serviceActionDoneMsg{message: fmt.Sprintf("Force deploy triggered: %s", svcName)}
					}
				case "update-count":
					return v, func() tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
						defer cancel()
						err := client.UpdateDesiredCount(ctx, cluster, svcName, count)
						if err != nil {
							return ErrorMsg{Err: err}
						}
						return serviceActionDoneMsg{message: fmt.Sprintf("Desired count updated to %d: %s", count, svcName)}
					}
				case "enable-metrics":
					v.metricsEnabled = true
					v.rebuildTable() // Show metric columns immediately with "..." while loading
					return v, tea.Batch(
						v.fetchMetrics(),
						v.fetchMetricsHistory(),
						func() tea.Msg { return StatusMsg{Message: "Metrics enabled (press r to refresh)"} },
					)
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
				taskView := NewTaskView(v.client, v.cluster, svc.Name, v.profile, v.region, v.readOnly, v.refreshInterval, v.shell)
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
		case "D":
			svc := v.selectedService()
			if svc != nil {
				deployView := NewDeploymentView(v.client, v.cluster, svc.Name, v.refreshInterval)
				return v, func() tea.Msg {
					return PushViewMsg{View: deployView}
				}
			}
		case "m":
			if v.metricsEnabled {
				v.metricsEnabled = false
				v.metrics = make(map[string]*awsclient.ServiceMetrics)
				v.metricsHist = make(map[string]*awsclient.ServiceMetricsHistory)
				v.rebuildTable()
				return v, func() tea.Msg {
					return StatusMsg{Message: "Metrics disabled"}
				}
			}
			svcCount := len(v.services)
			costPerRefresh := float64(svcCount*2) / 1000.0 * 0.01
			v.confirmAction = "enable-metrics"
			v.confirmMsg = fmt.Sprintf(
				"Enable CloudWatch metrics for %d services?\n"+
					"  Cost: ~$%.4f per refresh (manual only)\n"+
					"  Estimated: ~$%.2f/month at 100 refreshes/day\n"+
					"Enable? (y/n)",
				svcCount, costPerRefresh, costPerRefresh*100*22)
			return v, nil
		case "f":
			if v.readOnly {
				return v, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("action blocked: read-only mode")}
				}
			}
			svc := v.selectedService()
			if svc != nil {
				v.pendingService = svc.Name
				v.confirmAction = "force-deploy"
				v.confirmMsg = fmt.Sprintf("Force new deployment for '%s'? (y/n)", svc.Name)
			}
			return v, nil
		case "d":
			if v.readOnly {
				return v, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("action blocked: read-only mode")}
				}
			}
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
			cmds := []tea.Cmd{v.fetchServices()}
			if v.metricsEnabled {
				cmds = append(cmds, v.fetchMetrics(), v.fetchMetricsHistory())
			}
			return v, tea.Batch(cmds...)
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

	// Last updated indicator
	if !v.lastUpdated.IsZero() {
		ago := time.Since(v.lastUpdated).Truncate(time.Second)
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(
			fmt.Sprintf("  Updated %s ago", ago)))
		sb.WriteString("\n")
	}

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

	// Empty state
	if len(v.services) == 0 {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render(
			"  No services found in this cluster.\n  Press Esc to go back or r to refresh."))
		return sb.String()
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
		{Title: "Service", MinWidth: 12, Flex: 3},
		{Title: "Status", MinWidth: 8, Flex: 0},
		{Title: "Deploy", MinWidth: 5, Flex: 0},
		{Title: "Run/Des", MinWidth: 7, Flex: 0},
		{Title: "CPU Res", MinWidth: 7, Flex: 0},
		{Title: "Mem Res", MinWidth: 7, Flex: 0},
	}
	// CPU%/Mem% columns only when metrics enabled — saves space otherwise
	cpuColIdx, memColIdx := -1, -1
	if v.metricsEnabled {
		cpuColIdx = len(rcols)
		rcols = append(rcols, responsiveColumn{Title: "CPU %", MinWidth: 7, Flex: 1})
		memColIdx = len(rcols)
		rcols = append(rcols, responsiveColumn{Title: "Mem %", MinWidth: 7, Flex: 1})
	}
	rcols = append(rcols, responsiveColumn{Title: "Last Event", MinWidth: 15, Flex: 4})

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

		// Deployment status
		deployLabel := "..."
		if di, ok := v.deployments[svc.Name]; ok {
			deployLabel = awsclient.DeploymentStatusLabel(di.Deployments)
		}

		runDes := fmt.Sprintf("%d/%d", svc.RunningCount, svc.DesiredCount)

		row := table.Row{
			svc.Name,
			svc.Status,
			deployLabel,
			runDes,
			cpuRes,
			memRes,
		}

		if v.metricsEnabled {
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
			// Prepend sparkline if history available — width-aware to handle CJK terminals
			if h, ok := v.metricsHist[svc.Name]; ok {
				if len(h.CPUValues) > 0 && cpuPct != "..." {
					cpuPct = SparklineFit(h.CPUValues, widths[cpuColIdx], cpuPct)
				}
				if len(h.MemoryValues) > 0 && memPct != "..." {
					memPct = SparklineFit(h.MemoryValues, widths[memColIdx], memPct)
				}
			}
			row = append(row, cpuPct, memPct)
		}

		row = append(row, svc.LastEvent)
		rows = append(rows, row)
	}

	tableHeight := v.height - 3 // -1 for updated line, -2 for table padding
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
