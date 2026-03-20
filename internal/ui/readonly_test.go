package ui

import (
	"testing"
	"time"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	tea "github.com/charmbracelet/bubbletea"
)

// --- ServiceView Read-Only Tests ---

func newTestServiceView(readOnly bool) *ServiceView {
	return NewServiceView(nil, "cluster", "", "", readOnly, 10*time.Second, "/bin/sh", false, nil)
}

func TestServiceView_ReadOnly_BlocksForceDeploy(t *testing.T) {
	sv := newTestServiceView(true)
	sv.loaded = true
	sv.width = 100
	sv.height = 40

	_, cmd := sv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd == nil {
		t.Fatal("expected a command for blocked action")
	}
	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if !searchString(errMsg.Err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", errMsg.Err)
	}
}

func TestServiceView_ReadOnly_BlocksDesiredCount(t *testing.T) {
	sv := newTestServiceView(true)
	sv.loaded = true
	sv.width = 100
	sv.height = 40

	_, cmd := sv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected a command for blocked action")
	}
	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if !searchString(errMsg.Err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", errMsg.Err)
	}
}

func TestServiceView_Normal_AllowsForceDeploy(t *testing.T) {
	sv := newTestServiceView(false)
	sv.loaded = true
	sv.width = 100
	sv.height = 40

	_, cmd := sv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd != nil {
		msg := cmd()
		if errMsg, ok := msg.(ErrorMsg); ok {
			if searchString(errMsg.Err.Error(), "read-only") {
				t.Error("normal mode should not block with read-only error")
			}
		}
	}
}

func TestServiceView_ReadOnly_ShortcutHelp(t *testing.T) {
	sv := newTestServiceView(true)
	shortcuts := sv.ShortcutHelp()

	for _, s := range shortcuts {
		if s.Desc == "Force Deploy" {
			t.Error("read-only mode should not show Force Deploy shortcut")
		}
		if s.Desc == "Desired Count" {
			t.Error("read-only mode should not show Desired Count shortcut")
		}
	}

	found := map[string]bool{"Tasks": false, "Events": false, "Filter": false, "Refresh": false, "Back": false}
	for _, s := range shortcuts {
		if _, ok := found[s.Desc]; ok {
			found[s.Desc] = true
		}
	}
	for desc, present := range found {
		if !present {
			t.Errorf("read-only mode should still show %q shortcut", desc)
		}
	}
}

func TestServiceView_Normal_ShortcutHelp(t *testing.T) {
	sv := newTestServiceView(false)
	shortcuts := sv.ShortcutHelp()

	found := map[string]bool{"Force Deploy": false, "Desired Count": false}
	for _, s := range shortcuts {
		if _, ok := found[s.Desc]; ok {
			found[s.Desc] = true
		}
	}
	for desc, present := range found {
		if !present {
			t.Errorf("normal mode should show %q shortcut", desc)
		}
	}
}

// --- TaskView Read-Only Tests ---

func newTestTaskView(readOnly bool) *TaskView {
	return NewTaskView(nil, "cluster", "service", "", "", readOnly, 10*time.Second, "/bin/sh")
}

func TestTaskView_ReadOnly_BlocksStopTask(t *testing.T) {
	tv := newTestTaskView(true)
	tv.loaded = true
	tv.width = 100
	tv.height = 40

	_, cmd := tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("expected a command for blocked action")
	}
	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if !searchString(errMsg.Err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", errMsg.Err)
	}
}

func TestTaskView_ReadOnly_BlocksExec(t *testing.T) {
	tv := newTestTaskView(true)
	tv.loaded = true
	tv.width = 100
	tv.height = 40

	_, cmd := tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if cmd == nil {
		t.Fatal("expected a command for blocked action")
	}
	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if !searchString(errMsg.Err.Error(), "read-only") {
		t.Errorf("error should mention read-only, got: %v", errMsg.Err)
	}
}

func TestTaskView_ReadOnly_AllowsFilter(t *testing.T) {
	tv := newTestTaskView(true)
	tv.loaded = true
	tv.width = 100
	tv.height = 40

	tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !tv.filtering {
		t.Error("read-only mode should still allow filtering")
	}
}

func TestTaskView_ReadOnly_ShortcutHelp(t *testing.T) {
	tv := newTestTaskView(true)
	shortcuts := tv.ShortcutHelp()

	for _, s := range shortcuts {
		if s.Desc == "Exec" {
			t.Error("read-only mode should not show Exec shortcut")
		}
		if s.Desc == "Stop Task" {
			t.Error("read-only mode should not show Stop Task shortcut")
		}
	}

	found := map[string]bool{"Detail": false, "Logs": false, "Status Filter": false, "Filter": false, "Refresh": false, "Back": false}
	for _, s := range shortcuts {
		if _, ok := found[s.Desc]; ok {
			found[s.Desc] = true
		}
	}
	for desc, present := range found {
		if !present {
			t.Errorf("read-only mode should still show %q shortcut", desc)
		}
	}
}

func TestTaskView_Normal_ShortcutHelp(t *testing.T) {
	tv := newTestTaskView(false)
	shortcuts := tv.ShortcutHelp()

	found := map[string]bool{"Exec": false, "Stop Task": false, "Status Filter": false}
	for _, s := range shortcuts {
		if _, ok := found[s.Desc]; ok {
			found[s.Desc] = true
		}
	}
	for desc, present := range found {
		if !present {
			t.Errorf("normal mode should show %q shortcut", desc)
		}
	}
}

// --- Task Status Filter Tests ---

func TestTaskView_StatusFilterToggle(t *testing.T) {
	tv := newTestTaskView(false)
	tv.loaded = true
	tv.width = 100
	tv.height = 40

	// Default: RUNNING
	if tv.taskStatusFilter != ecstypes.DesiredStatusRunning {
		t.Errorf("default filter should be RUNNING, got %q", tv.taskStatusFilter)
	}
	if tv.statusFilterLabel() != "RUNNING" {
		t.Errorf("label should be RUNNING, got %q", tv.statusFilterLabel())
	}

	// Press t -> STOPPED
	tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if tv.taskStatusFilter != ecstypes.DesiredStatusStopped {
		t.Errorf("after first toggle should be STOPPED, got %q", tv.taskStatusFilter)
	}
	if tv.statusFilterLabel() != "STOPPED" {
		t.Errorf("label should be STOPPED, got %q", tv.statusFilterLabel())
	}

	// Press t -> ALL
	tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if tv.taskStatusFilter != "" {
		t.Errorf("after second toggle should be empty (ALL), got %q", tv.taskStatusFilter)
	}
	if tv.statusFilterLabel() != "ALL" {
		t.Errorf("label should be ALL, got %q", tv.statusFilterLabel())
	}

	// Press t -> RUNNING
	tv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if tv.taskStatusFilter != ecstypes.DesiredStatusRunning {
		t.Errorf("after third toggle should be RUNNING, got %q", tv.taskStatusFilter)
	}
}

// --- Refresh Interval Tests ---

func TestServiceView_RefreshDisabled(t *testing.T) {
	sv := NewServiceView(nil, "cluster", "", "", false, -1, "/bin/sh", false, nil)
	cmd := sv.tickCmd()
	if cmd != nil {
		t.Error("tickCmd should return nil when refresh is disabled")
	}
}

func TestServiceView_RefreshCustomInterval(t *testing.T) {
	sv := NewServiceView(nil, "cluster", "", "", false, 30*time.Second, "/bin/sh", false, nil)
	cmd := sv.tickCmd()
	if cmd == nil {
		t.Error("tickCmd should return a command for 30s interval")
	}
}

func TestTaskView_RefreshDisabled(t *testing.T) {
	tv := NewTaskView(nil, "cluster", "service", "", "", false, -1, "/bin/sh")
	cmd := tv.tickCmd()
	if cmd != nil {
		t.Error("tickCmd should return nil when refresh is disabled")
	}
}

func TestTaskView_RefreshCustomInterval(t *testing.T) {
	tv := NewTaskView(nil, "cluster", "service", "", "", false, 30*time.Second, "/bin/sh")
	cmd := tv.tickCmd()
	if cmd == nil {
		t.Error("tickCmd should return a command for 30s interval")
	}
}
