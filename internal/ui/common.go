package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsclient "github.com/soominna/ecs-tui/internal/aws"
)

// apiTimeout is the default timeout for AWS API calls.
const apiTimeout = 30 * time.Second

// View is the interface all views must implement.
type View interface {
	tea.Model
	ShortcutHelp() []Shortcut
	Title() string
}

// Closeable is optionally implemented by views that need cleanup on removal.
type Closeable interface {
	Close()
}

// Shortcut represents a key binding hint shown in the footer.
type Shortcut struct {
	Key  string
	Desc string
}

// --- Messages ---

// PushViewMsg pushes a new view onto the stack.
type PushViewMsg struct {
	View View
}

// PopViewMsg pops the current view from the stack.
type PopViewMsg struct{}

// ErrorMsg displays an error in the header.
type ErrorMsg struct {
	Err error
}

// StatusMsg displays a status message in the header.
type StatusMsg struct {
	Message string
}

// ClearErrorMsg clears the error display.
type ClearErrorMsg struct{}

// AWSConfigChangedMsg signals that the AWS profile/region was changed.
type AWSConfigChangedMsg struct {
	Profile string
	Region  string
}

// ClusterSelectedMsg is sent when a cluster is selected.
type ClusterSelectedMsg struct {
	ClusterName string
}

// AccessDeniedMsg signals an IAM permission error with guidance.
type AccessDeniedMsg struct {
	Err error
}

// themeChangedMsg signals that the dark/light theme was toggled.
type themeChangedMsg struct{}

// taskDefCacheUpdateMsg updates the app-level task definition cache.
type taskDefCacheUpdateMsg struct {
	Defs map[string]*awsclient.TaskDefinitionInfo
}

// awsClientReadyMsg signals that a new AWS client was created successfully.
type awsClientReadyMsg struct {
	Client  *awsclient.Client
	Profile string
	Region  string
}

// awsClientErrorMsg signals that AWS client creation failed.
type awsClientErrorMsg struct {
	Err error
}
