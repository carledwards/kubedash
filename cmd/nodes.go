package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
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
	allNodeData       map[string]NodeData            // Store complete node data
	allPodData        map[string]map[string][]string // Store complete pod data
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
		allNodeData:       make(map[string]NodeData),
		allPodData:        make(map[string]map[string][]string),
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

// SetAllData stores the complete node and pod data
func (nv *NodeView) SetAllData(nodeData map[string]NodeData, podData map[string]map[string][]string) {
	nv.allNodeData = nodeData
	nv.allPodData = podData
}

// GetFilteredData returns filtered node and pod data based on the search query
func (nv *NodeView) GetFilteredData(searchQuery string) (map[string]NodeData, map[string]map[string][]string) {
	if searchQuery == "" {
		return nv.allNodeData, nv.allPodData
	}

	searchQuery = strings.ToLower(searchQuery)
	filteredNodeData := make(map[string]NodeData)
	filteredPodData := make(map[string]map[string][]string)

	// Keep all nodes but filter their pods
	for nodeName, nodeData := range nv.allNodeData {
		// Create a copy of the node data
		filteredData := nodeData
		filteredData.Pods = make(map[string]PodInfo)

		// Filter pods for this node
		matchCount := 0
		matchingPods := make(map[string]bool) // Track matching pod names
		for podName, podInfo := range nodeData.Pods {
			if strings.Contains(strings.ToLower(podName), searchQuery) {
				filteredData.Pods[podName] = podInfo
				matchingPods[podName] = true
				matchCount++
			}
		}

		// Update pod count to show filtered/total
		if matchCount == nodeData.TotalPods {
			filteredData.PodCount = fmt.Sprintf("%d", matchCount)
		} else {
			filteredData.PodCount = fmt.Sprintf("%d (%d)", matchCount, nodeData.TotalPods)
		}

		filteredNodeData[nodeName] = filteredData

		// Filter pod indicators by namespace
		if podData, exists := nv.allPodData[nodeName]; exists {
			filteredPodData[nodeName] = make(map[string][]string)
			for namespace, indicators := range podData {
				var filteredIndicators []string
				// Get all pods for this namespace from the node data
				for podName := range nodeData.Pods {
					if podInfo, exists := nodeData.Pods[podName]; exists &&
						podInfo.Namespace == namespace &&
						matchingPods[podName] {
						// If the pod matches our search and belongs to this namespace,
						// keep its indicator
						for _, indicator := range indicators {
							if strings.Contains(strings.ToLower(podName), searchQuery) {
								filteredIndicators = append(filteredIndicators, indicator)
								break
							}
						}
					}
				}
				if len(filteredIndicators) > 0 {
					filteredPodData[nodeName][namespace] = filteredIndicators
				} else {
					// Ensure empty namespaces still show up in the table
					filteredPodData[nodeName][namespace] = []string{}
				}
			}
		}
	}

	return filteredNodeData, filteredPodData
}

// GetLastNodeData returns all stored node data
func (nv *NodeView) GetLastNodeData() map[string]NodeData {
	return nv.allNodeData
}

// GetLastPodData returns all stored pod data
func (nv *NodeView) GetLastPodData() map[string]map[string][]string {
	return nv.allPodData
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
		table.SetCell(startRow, 0, tview.NewTableCell(fmt.Sprintf("  %s: %s", k, v)))
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
