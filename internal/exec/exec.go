package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ExecDoneMsg is sent when exec-command completes.
type ExecDoneMsg struct {
	Err     error
	Hint    string // actionable fix command/instructions
	ErrType string // error category for UI routing
}

// ExecContainer uses aws ecs execute-command to attach to a container.
// It suspends the TUI, runs the subprocess, and resumes the TUI when done.
func ExecContainer(profile, region, cluster, service, taskID, container string) tea.Cmd {
	if taskID == "" {
		return func() tea.Msg {
			return ExecDoneMsg{Err: fmt.Errorf("no task ID available")}
		}
	}
	if container == "" {
		return func() tea.Msg {
			return ExecDoneMsg{Err: fmt.Errorf("no container name available (task may still be starting)")}
		}
	}

	// Validate arguments don't start with "-" to prevent argument injection
	for name, val := range map[string]string{
		"cluster": cluster, "task": taskID, "container": container,
		"profile": profile, "region": region,
	} {
		if val != "" && strings.HasPrefix(val, "-") {
			return func() tea.Msg {
				return ExecDoneMsg{Err: fmt.Errorf("invalid %s value: %q", name, val)}
			}
		}
	}

	// Check if aws CLI is available
	if _, err := exec.LookPath("aws"); err != nil {
		return func() tea.Msg {
			return ExecDoneMsg{Err: fmt.Errorf("aws CLI not found in PATH")}
		}
	}

	args := []string{
		"ecs", "execute-command",
		"--cluster", cluster,
		"--task", taskID,
		"--container", container,
		"--interactive",
		"--command", "/bin/sh",
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	if region != "" {
		args = append(args, "--region", region)
	}

	// Capture stderr so we can show the actual AWS error in the TUI
	// (alt screen restoration hides terminal output)
	var stderrBuf bytes.Buffer
	c := exec.Command("aws", args...)
	// Use raw os.Stdin/os.Stdout for the interactive session.
	// Bubbletea's internal cancelreader doesn't pass through properly
	// after ReleaseTerminal(), causing the session to hang.
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			captured := strings.TrimSpace(stderrBuf.String())
			msg := ExecDoneMsg{Err: fmt.Errorf("exec-command failed: %w", err)}

			if captured != "" {
				msg.Err = fmt.Errorf("%s", captured)
			}

			// Detect known errors and provide actionable hints
			lower := strings.ToLower(captured)
			switch {
			case strings.Contains(lower, "execute command was not enabled"):
				msg.ErrType = "exec-not-enabled"
				msg.Hint = buildEnableExecHint(profile, region, cluster, service)
			case strings.Contains(lower, "sessionmanagerplugin is not found"):
				msg.ErrType = "plugin-missing"
				msg.Hint = "Install the Session Manager Plugin:\n\n" +
					"  brew install --cask session-manager-plugin\n\n" +
					"Or see: https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html"
			case strings.Contains(lower, "targetnotconnectedexception"):
				msg.ErrType = "agent-not-running"
				msg.Hint = "The SSM agent is not running in the container.\n\n" +
					"Ensure the task role has the required SSM permissions\n" +
					"and the container has the SSM agent running."
			}

			return msg
		}
		return ExecDoneMsg{}
	})
}

// shellQuote wraps a string in single quotes for safe shell usage.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func buildEnableExecHint(profile, region, cluster, service string) string {
	var sb strings.Builder
	sb.WriteString("Execute command is not enabled for this service.\n")
	sb.WriteString("Run the following command to enable it:\n\n")

	sb.WriteString("  aws ecs update-service \\\n")
	sb.WriteString(fmt.Sprintf("    --cluster %s \\\n", shellQuote(cluster)))
	sb.WriteString(fmt.Sprintf("    --service %s \\\n", shellQuote(service)))
	sb.WriteString("    --enable-execute-command \\\n")
	sb.WriteString("    --force-new-deployment")
	if profile != "" {
		sb.WriteString(fmt.Sprintf(" \\\n    --profile %s", shellQuote(profile)))
	}
	if region != "" {
		sb.WriteString(fmt.Sprintf(" \\\n    --region %s", shellQuote(region)))
	}

	sb.WriteString("\n\nAfter the new tasks are RUNNING, try exec again.")
	return sb.String()
}
