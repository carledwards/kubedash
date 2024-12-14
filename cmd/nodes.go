package cmd

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeData represents information about a node and its pods
type NodeData struct {
	Name          string
	Status        string
	Version       string
	Age           string
	PodCount      string
	PodIndicators string
	Pods          map[string]PodInfo
	TotalPods     int
}

// CompareNodeData compares two NodeData instances for equality
func CompareNodeData(old, new NodeData) bool {
	if old.Status != new.Status ||
		old.Version != new.Version ||
		old.PodCount != new.PodCount ||
		old.PodIndicators != new.PodIndicators {
		return false
	}

	if len(old.Pods) != len(new.Pods) {
		return false
	}

	for podName, oldPod := range old.Pods {
		if newPod, exists := new.Pods[podName]; !exists || !ComparePodInfo(oldPod, newPod) {
			return false
		}
	}

	return true
}

// CompareNodes compares two node maps for equality
func CompareNodes(old, new map[string]NodeData) bool {
	if len(old) != len(new) {
		return false
	}

	for nodeName, oldNode := range old {
		if newNode, exists := new[nodeName]; !exists || !CompareNodeData(oldNode, newNode) {
			return false
		}
	}

	return true
}

// FormatDuration formats a duration in a human-readable format
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days >= 10 {
		return fmt.Sprintf("%dd", days)
	} else if days > 0 {
		if hours == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	}

	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, minutes)
}

// SetupNodeTable sets up the table headers and formatting
func SetupNodeTable(table *tview.Table) {
	table.SetFixed(1, 0)
	table.SetSelectable(true, true)
	table.SetSeparator(0)
	table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorNavy))
}

// Refreshable defines an interface for components that can be refreshed
type Refreshable interface {
	Refresh()
}

// NodeView represents the node table view
type NodeView struct {
	table             *tview.Table
	nodeMap           map[string]*corev1.Node
	includeNamespaces map[string]bool
	excludeNamespaces map[string]bool
	provider          *BaseK8sDataProvider
	rawNodeData       map[string]RawNodeData
	lastNodeData      map[string]NodeData
	lastPodData       map[string]map[string][]string
}

// NewNodeView creates a new NodeView instance
func NewNodeView(includeNs, excludeNs map[string]bool) *NodeView {
	table := tview.NewTable().
		SetBorders(false)

	SetupNodeTable(table)

	return &NodeView{
		table:             table,
		nodeMap:           make(map[string]*corev1.Node),
		includeNamespaces: includeNs,
		excludeNamespaces: excludeNs,
		rawNodeData:       make(map[string]RawNodeData),
		lastNodeData:      make(map[string]NodeData),
		lastPodData:       make(map[string]map[string][]string),
		provider:          &BaseK8sDataProvider{nodeMap: make(map[string]*corev1.Node)},
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

// GetVisibleNamespaces returns the map of visible namespaces
func (nv *NodeView) GetVisibleNamespaces() map[string]bool {
	return nv.includeNamespaces
}

// GetLastNodeData returns all stored node data
func (nv *NodeView) GetLastNodeData() map[string]NodeData {
	return nv.lastNodeData
}

// GetLastPodData returns all stored pod data
func (nv *NodeView) GetLastPodData() map[string]map[string][]string {
	return nv.lastPodData
}

// SetAllData stores the complete node and pod data
func (nv *NodeView) SetAllData(nodeData map[string]NodeData, podData map[string]map[string][]string) {
	// Store the last known state
	nv.lastNodeData = nodeData
	nv.lastPodData = podData

	// Update raw data for filtering
	nv.rawNodeData = make(map[string]RawNodeData)
	for nodeName, data := range nodeData {
		if node, exists := nv.nodeMap[nodeName]; exists {
			rawData := RawNodeData{
				Node: node,
				Pods: make(map[string]*corev1.Pod),
			}

			// Convert PodInfo back to raw pod data
			for podName, podInfo := range data.Pods {
				// Create a basic Pod object with the necessary fields for filtering
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: podInfo.Namespace,
					},
					Spec: corev1.PodSpec{
						NodeName: nodeName,
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPhase(podInfo.Status),
					},
				}
				rawData.Pods[podName] = pod
			}
			nv.rawNodeData[nodeName] = rawData
		}
	}
}

// GetFilteredData returns filtered node and pod data based on the search query
func (nv *NodeView) GetFilteredData(searchQuery string) (map[string]NodeData, map[string]map[string][]string) {
	criteria := FilterCriteria{
		IncludeNamespaces: nv.includeNamespaces,
		ExcludeNamespaces: nv.excludeNamespaces,
		SearchQuery:       searchQuery,
	}

	// Use the provider's filtering logic
	nodeData, podsByNode, _ := nv.provider.filterAndTransformData(nv.rawNodeData, criteria)
	return nodeData, podsByNode
}

// FormatMapAsRows formats a map as table rows
func FormatMapAsRows(table *tview.Table, startRow int, title string, m map[string]string) int {
	table.SetCell(startRow, 0, tview.NewTableCell(title).SetTextColor(tcell.ColorYellow))
	startRow++

	if len(m) == 0 {
		table.SetCell(startRow, 0, tview.NewTableCell("None").SetTextColor(tcell.ColorGray))
		return startRow + 1
	}

	for k, v := range m {
		table.SetCell(startRow, 0, tview.NewTableCell(fmt.Sprintf(DoubleSpace+"%s: %s", k, v)))
		startRow++
	}

	return startRow
}

// ComparePodInfo compares two PodInfo instances for equality
func ComparePodInfo(old, new PodInfo) bool {
	if old.Status != new.Status ||
		old.RestartCount != new.RestartCount ||
		len(old.ContainerInfo) != len(new.ContainerInfo) {
		return false
	}

	for containerName, oldContainer := range old.ContainerInfo {
		if newContainer, exists := new.ContainerInfo[containerName]; !exists ||
			oldContainer.Status != newContainer.Status ||
			oldContainer.RestartCount != newContainer.RestartCount {
			return false
		}
	}

	return true
}
