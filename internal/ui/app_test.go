package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewApp(t *testing.T) {
	t.Run("default mode", func(t *testing.T) {
		app := NewApp(nil, "", "", 0, "", false, false, "", "")
		if app.readOnly {
			t.Error("expected readOnly=false")
		}
		if app.refreshInterval != 10*time.Second {
			t.Errorf("expected default 10s interval, got %v", app.refreshInterval)
		}
		if app.shell != "/bin/sh" {
			t.Errorf("expected default shell /bin/sh, got %q", app.shell)
		}
	})

	t.Run("read-only mode", func(t *testing.T) {
		app := NewApp(nil, "my-cluster", "", 0, "", true, false, "", "")
		if !app.readOnly {
			t.Error("expected readOnly=true")
		}
	})

	t.Run("custom refresh interval", func(t *testing.T) {
		app := NewApp(nil, "", "", 30, "", false, false, "", "")
		if app.refreshInterval != 30*time.Second {
			t.Errorf("expected 30s interval, got %v", app.refreshInterval)
		}
	})

	t.Run("disabled refresh", func(t *testing.T) {
		app := NewApp(nil, "", "", -1, "", false, false, "", "")
		if app.refreshInterval != -1 {
			t.Errorf("expected -1 (disabled), got %v", app.refreshInterval)
		}
	})

	t.Run("custom shell", func(t *testing.T) {
		app := NewApp(nil, "", "", 0, "/bin/bash", false, false, "", "")
		if app.shell != "/bin/bash" {
			t.Errorf("shell = %q, want /bin/bash", app.shell)
		}
	})
}

func TestApp_HelpToggle(t *testing.T) {
	app := NewApp(nil, "", "", 0, "", false, false, "", "")
	app.width = 100
	app.height = 40
	app.Init()

	if app.showHelp {
		t.Error("help should be hidden initially")
	}

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !app.showHelp {
		t.Error("help should be visible after ? press")
	}

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if app.showHelp {
		t.Error("help should be hidden after second ? press")
	}
}

func TestApp_ReadOnlyHeader(t *testing.T) {
	app := NewApp(nil, "", "", 0, "", true, false, "", "")
	app.width = 120
	app.height = 40
	app.Init()

	view := app.View()
	if !containsText(view, "READ-ONLY") {
		t.Error("header should show READ-ONLY badge")
	}
}

func TestApp_NormalHeader(t *testing.T) {
	app := NewApp(nil, "", "", 0, "", false, false, "", "")
	app.width = 120
	app.height = 40
	app.Init()

	view := app.View()
	if containsText(view, "READ-ONLY") {
		t.Error("header should not show READ-ONLY badge in normal mode")
	}
}

// containsText checks if rendered output contains a text substring.
func containsText(rendered, text string) bool {
	clean := stripAnsi(rendered)
	return contains(clean, text)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}
