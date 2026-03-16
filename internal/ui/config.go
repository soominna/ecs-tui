package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	awsclient "github.com/nicewook/ecs-tui/internal/aws"
)

type configStep int

const (
	stepQuickStart configStep = iota
	stepProfile
	stepRegion
)

type ConfigView struct {
	step     configStep
	list     list.Model
	width    int
	height   int
	ready    bool
	profiles []string
	regions  []string

	// Detected/current settings
	currentProfile string
	currentRegion  string

	// User selections
	selectedProfile string
	selectedRegion  string

	// Previous client info (when invoked via P key)
	prevProfile string
	prevRegion  string
}

// configItem is a list item with title and optional description.
type configItem struct {
	title string
	desc  string
}

func (i configItem) Title() string       { return i.title }
func (i configItem) Description() string { return i.desc }
func (i configItem) FilterValue() string { return i.title }

type profilesLoadedMsg struct {
	profiles       []string
	currentProfile string
	currentRegion  string
}

// NewConfigView creates a config view for first-time setup.
func NewConfigView() *ConfigView {
	return &ConfigView{
		step:    stepQuickStart,
		regions: awsclient.CommonRegions(),
	}
}

// NewConfigViewWithCurrent creates a config view pre-populated with current settings.
// Used when switching profile/region via P key.
func NewConfigViewWithCurrent(profile, region string) *ConfigView {
	return &ConfigView{
		step:        stepQuickStart,
		regions:     awsclient.CommonRegions(),
		prevProfile: profile,
		prevRegion:  region,
	}
}

func (v *ConfigView) Title() string {
	switch v.step {
	case stepQuickStart:
		return "AWS Configuration"
	case stepProfile:
		return "Select Profile"
	case stepRegion:
		return "Select Region"
	}
	return "Configuration"
}

func (v *ConfigView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "Enter", Desc: "Select"},
		{Key: "/", Desc: "Filter"},
		{Key: "Esc", Desc: "Back"},
	}
}

func (v *ConfigView) Init() tea.Cmd {
	return func() tea.Msg {
		profiles, err := awsclient.ListProfiles()
		if err != nil || len(profiles) == 0 {
			profiles = []string{"default"}
		}
		curProfile, curRegion := awsclient.DetectCurrentConfig()
		return profilesLoadedMsg{
			profiles:       profiles,
			currentProfile: curProfile,
			currentRegion:  curRegion,
		}
	}
}

func (v *ConfigView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.ready {
			v.list.SetSize(v.width, v.listHeight())
		}
		return v, nil

	case profilesLoadedMsg:
		v.profiles = msg.profiles
		v.currentProfile = msg.currentProfile
		v.currentRegion = msg.currentRegion

		// If opened via P key, prefer the previous active values
		if v.prevProfile != "" {
			v.currentProfile = v.prevProfile
		}
		if v.prevRegion != "" {
			v.currentRegion = v.prevRegion
		}

		v.buildQuickStartList()
		v.ready = true
		return v, nil

	case tea.KeyMsg:
		if !v.ready {
			return v, nil
		}
		switch msg.String() {
		case "enter":
			item := v.list.SelectedItem()
			if item == nil {
				return v, nil
			}
			selected, ok := item.(configItem)
			if !ok {
				return v, nil
			}
			return v, v.handleSelection(selected)

		case "esc":
			return v, v.handleEsc()
		}
	}

	if v.ready {
		var cmd tea.Cmd
		v.list, cmd = v.list.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *ConfigView) handleSelection(selected configItem) tea.Cmd {
	switch v.step {
	case stepQuickStart:
		switch selected.title {
		case "Continue with current settings":
			return func() tea.Msg {
				return AWSConfigChangedMsg{
					Profile: v.currentProfile,
					Region:  v.currentRegion,
				}
			}
		case "Change profile":
			v.step = stepProfile
			v.buildProfileList()
		case "Change region":
			v.selectedProfile = v.currentProfile
			v.step = stepRegion
			v.buildRegionList()
		case "Change both":
			v.step = stepProfile
			v.buildProfileList()
		}

	case stepProfile:
		v.selectedProfile = selected.title
		v.step = stepRegion
		v.buildRegionList()

	case stepRegion:
		v.selectedRegion = selected.title
		return func() tea.Msg {
			return AWSConfigChangedMsg{
				Profile: v.selectedProfile,
				Region:  v.selectedRegion,
			}
		}
	}

	return nil
}

func (v *ConfigView) handleEsc() tea.Cmd {
	switch v.step {
	case stepRegion:
		v.step = stepProfile
		v.buildProfileList()
	case stepProfile:
		v.step = stepQuickStart
		v.buildQuickStartList()
	default:
		return func() tea.Msg { return PopViewMsg{} }
	}
	return nil
}

func (v *ConfigView) View() string {
	if !v.ready {
		return loadingStyle.Render("  Detecting AWS configuration...")
	}

	var sb strings.Builder

	// Render context box at the top
	sb.WriteString(v.renderContextBox())
	sb.WriteString("\n")

	// Step indicator
	if v.step == stepProfile {
		sb.WriteString(stepLabelStyle.Render("  Step 1/2 "))
		sb.WriteString(stepDescStyle.Render("Choose a profile"))
		sb.WriteString("\n")
	} else if v.step == stepRegion {
		sb.WriteString(stepLabelStyle.Render("  Step 2/2 "))
		sb.WriteString(stepDescStyle.Render(
			fmt.Sprintf("Choose a region  (Profile: %s)", v.selectedProfile)))
		sb.WriteString("\n")
	}

	sb.WriteString(v.list.View())
	return sb.String()
}

func (v *ConfigView) renderContextBox() string {
	profile := v.currentProfile
	region := v.currentRegion
	if v.selectedProfile != "" && v.step == stepRegion {
		profile = v.selectedProfile
	}
	if region == "" {
		region = "(not set)"
	}

	label := configLabelStyle.Render
	value := configValueStyle.Render

	content := fmt.Sprintf("  %s %s    %s %s",
		label("Profile:"), value(profile),
		label("Region:"), value(region),
	)

	return configBoxStyle.Width(v.width - 2).Render(content)
}

func (v *ConfigView) buildQuickStartList() {
	var items []list.Item

	desc := fmt.Sprintf("%s / %s", v.currentProfile, v.currentRegion)
	if v.currentRegion == "" {
		desc = fmt.Sprintf("%s / (auto-detect region)", v.currentProfile)
	}

	items = append(items, configItem{
		title: "Continue with current settings",
		desc:  desc,
	})
	items = append(items, configItem{
		title: "Change profile",
		desc:  "Select a different AWS profile",
	})
	items = append(items, configItem{
		title: "Change region",
		desc:  fmt.Sprintf("Keep profile %q, pick a different region", v.currentProfile),
	})
	items = append(items, configItem{
		title: "Change both",
		desc:  "Select profile and region",
	})

	v.list = v.newList(items, "AWS Configuration")
	v.list.SetFilteringEnabled(false)
}

func (v *ConfigView) buildProfileList() {
	var items []list.Item

	// Put current profile first with a marker
	for _, p := range v.profiles {
		desc := ""
		if p == v.currentProfile {
			desc = "current"
		}
		items = append(items, configItem{title: p, desc: desc})
	}

	// Move current to front if not already
	for i, item := range items {
		if item.(configItem).title == v.currentProfile && i != 0 {
			items[0], items[i] = items[i], items[0]
			break
		}
	}

	v.list = v.newList(items, "Select Profile")
	v.list.SetFilteringEnabled(true)
}

func (v *ConfigView) buildRegionList() {
	var items []list.Item

	// Put current region first
	for _, r := range v.regions {
		desc := ""
		if r == v.currentRegion {
			desc = "current"
		}
		items = append(items, configItem{title: r, desc: desc})
	}

	for i, item := range items {
		if item.(configItem).title == v.currentRegion && i != 0 {
			items[0], items[i] = items[i], items[0]
			break
		}
	}

	v.list = v.newList(items, "Select Region")
	v.list.SetFilteringEnabled(true)
}

func (v *ConfigView) newList(items []list.Item, title string) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		BorderLeftForeground(colorAccent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#AAAAAA")).
		BorderLeftForeground(colorAccent)

	l := list.New(items, delegate, v.width, v.listHeight())
	l.Title = title
	l.Styles.Title = titleStyle.Padding(0, 1)
	l.SetShowStatusBar(true)
	l.SetShowHelp(false)
	return l
}

func (v *ConfigView) listHeight() int {
	// Reserve space for context box (~3 lines) + step indicator (~1 line)
	h := v.height - 5
	if h < 5 {
		h = 5
	}
	return h
}

// Styles for the config view
var (
	configBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(0, 1)

	configLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Bold(true)

	configValueStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Bold(true)

	stepLabelStyle = lipgloss.NewStyle().
			Background(colorPrimary).
			Foreground(colorWhite).
			Bold(true).
			Padding(0, 1)

	stepDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Padding(0, 1)
)
