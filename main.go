package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
	cfgpkg "github.com/soominna/ecs-tui/internal/config"
	"github.com/soominna/ecs-tui/internal/ui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	profile := flag.String("profile", "", "AWS profile name")
	region := flag.String("region", "", "AWS region")
	cluster := flag.String("cluster", "", "ECS cluster name")
	service := flag.String("service", "", "ECS service name (requires --cluster)")
	readOnly := flag.Bool("read-only", false, "Read-only mode (disable all mutative actions)")
	refreshInterval := flag.Int("refresh", 0, "Auto-refresh interval in seconds (-1 to disable)")
	metricsEnabled := flag.Bool("metrics", false, "Enable CloudWatch metrics (costs apply)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ecs-tui version %s\n", version)
		os.Exit(0)
	}

	if *service != "" && *cluster == "" {
		fmt.Fprintln(os.Stderr, "Error: --service requires --cluster")
		os.Exit(1)
	}

	// Load config file
	cfg := cfgpkg.Load()

	// CLI flags override config values (only when explicitly set)
	flagSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { flagSet[f.Name] = true })

	if !flagSet["cluster"] && cfg.DefaultCluster != "" {
		*cluster = cfg.DefaultCluster
	}
	if !flagSet["service"] && cfg.DefaultService != "" {
		*service = cfg.DefaultService
	}
	if !flagSet["read-only"] && cfg.ReadOnly {
		*readOnly = true
	}
	if !flagSet["refresh"] {
		*refreshInterval = cfg.RefreshInterval
	}
	if !flagSet["metrics"] && cfg.Metrics {
		*metricsEnabled = true
	}

	// Apply theme from config
	if cfg.Theme != "" {
		ui.ApplyTheme(cfg.Theme)
	}

	var client awsclient.ECSAPI
	var err error

	// If no CLI flags, try restoring last session, then detect current config
	if *profile == "" && *region == "" && !flagSet["cluster"] && cfg.DefaultCluster == "" {
		if last := awsclient.LoadLastSession(); last != nil {
			*profile = last.Profile
			*region = last.Region
			if last.Theme != "" {
				ui.ApplyTheme(last.Theme)
			}
		} else {
			*profile, *region = awsclient.DetectCurrentConfig()
		}
	}

	if *profile != "" || *region != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		client, err = awsclient.NewClient(ctx, *profile, *region)
		if err != nil {
			// Don't exit — fall through to ConfigView so user can fix settings
			fmt.Fprintf(os.Stderr, "Warning: could not connect to AWS (%v), opening config view...\n", err)
			client = nil
		} else {
			if err := awsclient.SaveLastSession(*profile, *region, ui.CurrentThemeName()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not save session: %v\n", err)
			}
		}
	}

	app := ui.NewApp(client, *cluster, *service, *refreshInterval, cfg.Shell, *readOnly, *metricsEnabled, *profile, *region)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
