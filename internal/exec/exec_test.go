package exec

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"a'b'c", "'a'\\''b'\\''c'"},
	}

	for _, tt := range tests {
		got := shellQuote(tt.input)
		if got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildEnableExecHint(t *testing.T) {
	t.Run("with profile and region", func(t *testing.T) {
		hint := buildEnableExecHint("prod", "us-east-1", "my-cluster", "my-service")

		if !strings.Contains(hint, "Execute command is not enabled") {
			t.Error("hint should explain the issue")
		}
		if !strings.Contains(hint, "--cluster 'my-cluster'") {
			t.Error("hint should contain cluster name")
		}
		if !strings.Contains(hint, "--service 'my-service'") {
			t.Error("hint should contain service name")
		}
		if !strings.Contains(hint, "--profile 'prod'") {
			t.Error("hint should contain profile")
		}
		if !strings.Contains(hint, "--region 'us-east-1'") {
			t.Error("hint should contain region")
		}
		if !strings.Contains(hint, "--enable-execute-command") {
			t.Error("hint should contain enable flag")
		}
	})

	t.Run("without profile and region", func(t *testing.T) {
		hint := buildEnableExecHint("", "", "cluster", "service")

		if strings.Contains(hint, "--profile") {
			t.Error("hint should not contain --profile when empty")
		}
		if strings.Contains(hint, "--region") {
			t.Error("hint should not contain --region when empty")
		}
	})
}

func TestExecContainer_Validation(t *testing.T) {
	t.Run("empty task ID", func(t *testing.T) {
		cmd := ExecContainer("profile", "region", "cluster", "service", "", "container", "/bin/sh")
		msg := cmd()
		done, ok := msg.(ExecDoneMsg)
		if !ok {
			t.Fatal("expected ExecDoneMsg")
		}
		if done.Err == nil {
			t.Fatal("expected error for empty task ID")
		}
		if !strings.Contains(done.Err.Error(), "no task ID") {
			t.Errorf("unexpected error: %v", done.Err)
		}
	})

	t.Run("empty container", func(t *testing.T) {
		cmd := ExecContainer("profile", "region", "cluster", "service", "task-123", "", "/bin/sh")
		msg := cmd()
		done, ok := msg.(ExecDoneMsg)
		if !ok {
			t.Fatal("expected ExecDoneMsg")
		}
		if done.Err == nil {
			t.Fatal("expected error for empty container")
		}
		if !strings.Contains(done.Err.Error(), "no container name") {
			t.Errorf("unexpected error: %v", done.Err)
		}
	})

	t.Run("argument injection prevention", func(t *testing.T) {
		cmd := ExecContainer("profile", "region", "-malicious", "service", "task-123", "container", "/bin/sh")
		msg := cmd()
		done, ok := msg.(ExecDoneMsg)
		if !ok {
			t.Fatal("expected ExecDoneMsg")
		}
		if done.Err == nil {
			t.Fatal("expected error for injection attempt")
		}
		if !strings.Contains(done.Err.Error(), "invalid") {
			t.Errorf("unexpected error: %v", done.Err)
		}
	})
}
