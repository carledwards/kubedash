package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
)

// PodInfo represents the state of a pod
type PodInfo struct {
	Name          string
	Status        string
	RestartCount  int
	ContainerInfo map[string]ContainerInfo
}

// ContainerInfo represents the state of a container
type ContainerInfo struct {
	Status       string
	RestartCount int
}

// NodeData represents the state of a node and its pods
type NodeData struct {
	Name          string
	Status        string
	Version       string
	PodCount      string // Will now show as "filtered (total)"
	Age           string
	PodIndicators string
	Pods          map[string]PodInfo // Map of pod name to pod info
	TotalPods     int                // Track total pods before filtering
}

// CompareNodeData performs a deep comparison of two NodeData instances
func CompareNodeData(old, new NodeData) bool {
	if old.Name != new.Name ||
		old.Status != new.Status ||
		old.Version != new.Version ||
		old.PodCount != new.PodCount ||
		old.Age != new.Age ||
		old.TotalPods != new.TotalPods ||
		old.PodIndicators != new.PodIndicators {
		return true
	}

	if len(old.Pods) != len(new.Pods) {
		return true
	}

	for podName, oldPod := range old.Pods {
		if newPod, exists := new.Pods[podName]; !exists {
			return true
		} else if ComparePodInfo(oldPod, newPod) {
			return true
		}
	}

	return false
}

// CompareNodes checks if two node states are different
func CompareNodes(old, new map[string]NodeData) bool {
	if len(old) != len(new) {
		return true
	}
	for name, oldData := range old {
		if newData, exists := new[name]; !exists {
			return true
		} else if CompareNodeData(oldData, newData) {
			return true
		}
	}
	return false
}

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24

	if days >= 10 {
		return fmt.Sprintf("%dd", days)
	} else if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := int(d.Minutes())
	return fmt.Sprintf("%dm", minutes)
}

// SetupNodeTable initializes a table with node headers
func SetupNodeTable(table *tview.Table) {
	headers := []string{"Node Name", "Status", "Version", "Age", "PODS"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)

		// Right-align Age and PODS columns
		if header == "Age" || header == "PODS" {
			cell.SetAlign(tview.AlignRight)
		}

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

// ComparePodInfo compares two PodInfo instances and returns true if they differ
func ComparePodInfo(old, new PodInfo) bool {
	if old.Status != new.Status || old.RestartCount != new.RestartCount {
		return true
	}

	if len(old.ContainerInfo) != len(new.ContainerInfo) {
		return true
	}

	for containerName, oldContainer := range old.ContainerInfo {
		if newContainer, exists := new.ContainerInfo[containerName]; !exists {
			return true
		} else if oldContainer != newContainer {
			return true
		}
	}

	return false
}
