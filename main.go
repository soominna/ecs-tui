package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
	"github.com/soominna/ecs-tui/internal/ui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	profile := flag.String("profile", "", "AWS profile name")
	region := flag.String("region", "", "AWS region")
	cluster := flag.String("cluster", "", "ECS cluster name")
	service := flag.String("service", "", "ECS service name (requires --cluster)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("ecs-tui version %s\n", version)
		os.Exit(0)
	}

	if *service != "" && *cluster == "" {
		fmt.Fprintln(os.Stderr, "Error: --service requires --cluster")
		os.Exit(1)
	}

	var client *awsclient.Client
	var err error

	// If no CLI flags, try restoring last session, then detect current config
	if *profile == "" && *region == "" && *cluster == "" {
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
		client, err = awsclient.NewClient(context.Background(), *profile, *region)
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

	app := ui.NewApp(client, *cluster, *service)

	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
