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
)

// Time intervals
const (
	RefreshInterval = 10 * time.Second
	APITimeout      = 30 * time.Second
)
