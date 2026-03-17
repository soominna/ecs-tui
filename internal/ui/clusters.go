package ui

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

type ClusterView struct {
	client   *awsclient.Client
	list     list.Model
	clusters []awsclient.ClusterInfo
	width    int
	height   int
	loaded   bool
}

type clusterItem struct {
	info awsclient.ClusterInfo
}

func (i clusterItem) Title() string       { return i.info.Name }
func (i clusterItem) Description() string { return i.info.ARN }
func (i clusterItem) FilterValue() string { return i.info.Name }

type clustersLoadedMsg struct {
	clusters []awsclient.ClusterInfo
}

func NewClusterView(client *awsclient.Client) *ClusterView {
	return &ClusterView{client: client}
}

func (v *ClusterView) Title() string { return "Clusters" }

func (v *ClusterView) ShortcutHelp() []Shortcut {
	return []Shortcut{
		{Key: "Enter", Desc: "Select"},
		{Key: "/", Desc: "Filter"},
		{Key: "r", Desc: "Refresh"},
	}
}

func (v *ClusterView) Init() tea.Cmd {
	return v.fetchClusters()
}

func (v *ClusterView) fetchClusters() tea.Cmd {
	client := v.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		clusters, err := client.ListClusters(ctx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return clustersLoadedMsg{clusters: clusters}
	}
}

func (v *ClusterView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.loaded {
			v.list.SetSize(v.width, v.height)
		}
		return v, nil

	case clustersLoadedMsg:
		v.clusters = msg.clusters
		v.rebuildList()
		v.loaded = true
		return v, nil

	case themeChangedMsg:
		if v.loaded {
			v.rebuildList()
		}
		return v, nil

	case tea.KeyMsg:
		if !v.loaded {
			return v, nil
		}
		switch msg.String() {
		case "enter":
			item := v.list.SelectedItem()
			if item == nil {
				return v, nil
			}
			selected, ok := item.(clusterItem)
			if !ok {
				return v, nil
			}
			return v, func() tea.Msg {
				return ClusterSelectedMsg{ClusterName: selected.info.Name}
			}
		case "r":
			v.loaded = false
			return v, v.fetchClusters()
		case "esc":
			return v, func() tea.Msg { return PopViewMsg{} }
		}
	}

	if v.loaded {
		var cmd tea.Cmd
		v.list, cmd = v.list.Update(msg)
		return v, cmd
	}
	return v, nil
}

func (v *ClusterView) rebuildList() {
	var items []list.Item
	for _, cl := range v.clusters {
		items = append(items, clusterItem{info: cl})
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colorText).
		Background(colorSurface0).
		BorderLeftForeground(colorBlue)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colorSubtext0).
		Background(colorSurface0).
		BorderLeftForeground(colorBlue)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(colorSubtext1)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(colorOverlay1)
	v.list = list.New(items, delegate, v.width, v.height)
	v.list.Title = "ECS Clusters"
	v.list.Styles.Title = titleStyle.Padding(0, 1)
	v.list.SetShowStatusBar(true)
	v.list.SetFilteringEnabled(true)
	v.list.SetShowHelp(false)
}

func (v *ClusterView) View() string {
	if !v.loaded {
		return loadingStyle.Render("  Loading clusters...")
	}
	return v.list.View()
}
