package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
)

// NodeData represents the state of a node and its pods
type NodeData struct {
	Name          string
	Status        string
	Version       string
	PodCount      string
	Age           string
	PodIndicators string
}

// CompareNodes checks if two node states are different
func CompareNodes(old, new map[string]NodeData) bool {
	if len(old) != len(new) {
		return true
	}
	for name, oldData := range old {
		if newData, exists := new[name]; !exists {
			return true
		} else if oldData != newData {
			return true
		}
	}
	return false
}

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := int(d.Minutes())
	return fmt.Sprintf("%dm", minutes)
}

// SetupNodeTable initializes a table with node headers
func SetupNodeTable(table *tview.Table) {
	headers := []string{"Node Name", "Status", "Version", "PODS", "Age", "Pods"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}
}

// Refreshable interface defines the contract for components that can be refreshed
type Refreshable interface {
	Refresh() error
}

// NodeView represents the main node table view
type NodeView struct {
	table             *tview.Table
	nodeMap           map[string]*corev1.Node
	includeNamespaces map[string]bool
	excludeNamespaces map[string]bool
	visibleNamespaces map[string]bool
}

// NewNodeView creates a new NodeView instance
func NewNodeView(includeNs, excludeNs map[string]bool) *NodeView {
	table := tview.NewTable().
		SetFixed(1, 0).
		SetBorders(false).
		SetSelectable(true, true).
		SetSelectedStyle(tcell.StyleDefault.
			Background(tcell.ColorNavy).
			Foreground(tcell.ColorWhite))

	SetupNodeTable(table)

	return &NodeView{
		table:             table,
		nodeMap:           make(map[string]*corev1.Node),
		includeNamespaces: includeNs,
		excludeNamespaces: excludeNs,
		visibleNamespaces: make(map[string]bool),
	}
}

// GetTable returns the underlying table
func (nv *NodeView) GetTable() *tview.Table {
	return nv.table
}

// GetNodeMap returns the node map
func (nv *NodeView) GetNodeMap() map[string]*corev1.Node {
	return nv.nodeMap
}

// GetVisibleNamespaces returns the visible namespaces
func (nv *NodeView) GetVisibleNamespaces() map[string]bool {
	return nv.visibleNamespaces
}

// FormatMapAsRows formats a map as table rows
func FormatMapAsRows(table *tview.Table, startRow int, title string, m map[string]string) int {
	if len(m) == 0 {
		table.SetCell(startRow, 0, tview.NewTableCell(title).SetTextColor(tcell.ColorYellow))
		table.SetCell(startRow, 1, tview.NewTableCell("None").SetTextColor(tcell.ColorWhite))
		return startRow + 1
	}

	table.SetCell(startRow, 0, tview.NewTableCell(title).SetTextColor(tcell.ColorYellow))
	startRow++

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		table.SetCell(startRow, 0, tview.NewTableCell("  "+k).SetTextColor(tcell.ColorSkyblue))
		table.SetCell(startRow, 1, tview.NewTableCell(m[k]).SetTextColor(tcell.ColorWhite))
		startRow++
	}
	return startRow
}
