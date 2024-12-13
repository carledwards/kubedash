package cmd

import "time"

// Pod status indicators with colors
const (
	PodIndicatorGreen  = "[green]■[white] "
	PodIndicatorYellow = "[yellow]■[white] "
	PodIndicatorRed    = "[red]■[white] "
)

// Color names for sorting and comparison
const (
	ColorRed    = "red"
	ColorYellow = "yellow"
	ColorGreen  = "green"
	ColorWhite  = "white"
)

// Color tags for text formatting
const (
	ColorTagRed    = "[red]"
	ColorTagYellow = "[yellow]"
	ColorTagGreen  = "[green]"
	ColorTagWhite  = "[white]"
)

// Pod status strings
const (
	PodStatusRunning     = "Running"
	PodStatusPending     = "Pending"
	PodStatusTerminating = "Terminating"
	PodStatusUnknown     = "Unknown"
)

// Node status strings
const (
	NodeStatusReady    = "Ready"
	NodeStatusNotReady = "NotReady"
)

// Keyboard commands
const (
	KeyRefresh      = 'r'
	KeyClearHistory = 'c'
	KeyHelp         = '?'
)

// Dialog text
const (
	ErrorDialogText = "Unable to fetch Kubernetes data.\nCheck your network connection.\nWill retry automatically."

	HelpDialogText = `Keyboard Shortcuts:

[yellow]?[white] - Show this help
[yellow]r[white] - Refresh data
[yellow]c[white] - Clear changelog
[yellow]Tab[white] - Switch between main view and changelog
[yellow]Enter[white] - Show node details
[yellow]Esc[white] - Close details view or help
[yellow]↑/↓/←/→[white] - Navigate tables
[yellow]PgUp/PgDn[white] - Page up/down in details view
[yellow]Home/End[white] - Jump to top/bottom in details view`
)

// Time intervals
const (
	RefreshInterval = 10 * time.Second
	APITimeout      = 30 * time.Second
)
