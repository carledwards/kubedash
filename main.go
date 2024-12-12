package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	// Handle comma-separated values
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			*i = append(*i, trimmed)
		}
	}
	return nil
}

// nodeData represents the state of a node and its pods
type nodeData struct {
	Name          string
	Status        string
	Version       string
	PodCount      string
	Age           string
	PodIndicators string
}

// compareNodes checks if two node states are different
func compareNodes(old, new map[string]nodeData) bool {
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

// formatDuration formats a duration in a human-readable format
func formatDuration(d time.Duration) string {
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

// Helper function to format map as table rows
func formatMapAsRows(table *tview.Table, startRow int, title string, m map[string]string) int {
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

// updateNodeData updates the table with fresh node and pod data
func updateNodeData(clientset *kubernetes.Clientset, table *tview.Table, nodeMap map[string]*corev1.Node, includeNamespaces, excludeNamespaces, visibleNamespaces map[string]bool) error {
	// Store current state
	oldState := make(map[string]nodeData)
	for row := 1; row < table.GetRowCount(); row++ {
		name := table.GetCell(row, 0).Text
		oldState[name] = nodeData{
			Name:          name,
			Status:        table.GetCell(row, 1).Text,
			Version:       table.GetCell(row, 2).Text,
			PodCount:      table.GetCell(row, 3).Text,
			Age:           table.GetCell(row, 4).Text,
			PodIndicators: table.GetCell(row, 5).Text,
		}
	}

	// Get nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	// Build new state
	newState := make(map[string]nodeData)
	newNodeMap := make(map[string]*corev1.Node)
	newVisibleNamespaces := make(map[string]bool)

	// Process nodes
	for _, node := range nodes.Items {
		nodeCopy := node
		newNodeMap[node.Name] = &nodeCopy

		// Get node status
		status := "Not Ready"
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" {
				if cond.Status == "True" {
					status = "Ready"
				}
				break
			}
		}

		// Get pods for this node
		pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + node.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods for node %s: %v", node.Name, err)
		}

		// Count total and filtered pods
		totalPods := len(pods.Items)
		filteredPods := 0
		var podIndicators []string
		for _, pod := range pods.Items {
			// Skip if namespace is excluded
			if excludeNamespaces[pod.Namespace] {
				continue
			}
			// Skip if we have includes and this namespace isn't included
			if len(includeNamespaces) > 0 && !includeNamespaces[pod.Namespace] {
				continue
			}

			filteredPods++

			// Add namespace to visible set
			newVisibleNamespaces[pod.Namespace] = true

			// Check pod restarts
			var restarts int32
			for _, containerStatus := range pod.Status.ContainerStatuses {
				restarts += containerStatus.RestartCount
			}

			// Color based on restarts with consistent spacing
			if restarts == 0 {
				podIndicators = append(podIndicators, "[green]■[white] ")
			} else {
				podIndicators = append(podIndicators, "[yellow]■[white] ")
			}
		}

		// Format pod count string
		podCountStr := fmt.Sprintf("%d", totalPods)
		if len(includeNamespaces) > 0 || len(excludeNamespaces) > 0 {
			podCountStr = fmt.Sprintf("%d (%d)", filteredPods, totalPods)
		}

		// Calculate node age
		age := formatDuration(time.Since(node.CreationTimestamp.Time))

		// Store new state
		newState[node.Name] = nodeData{
			Name:          node.Name,
			Status:        status,
			Version:       node.Status.NodeInfo.KubeletVersion,
			PodCount:      podCountStr,
			Age:           age,
			PodIndicators: strings.Join(podIndicators, ""),
		}
	}

	// Check if there are any changes
	if !compareNodes(oldState, newState) {
		return nil // No changes, don't update display
	}

	// Clear existing data but preserve headers
	table.Clear()

	// Re-add headers
	headers := []string{"Node Name", "Status", "Version", "PODS", "Age", "Pods"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}

	// Update node map
	for k := range nodeMap {
		delete(nodeMap, k)
	}
	for k, v := range newNodeMap {
		nodeMap[k] = v
	}

	// Update visible namespaces
	for k := range visibleNamespaces {
		delete(visibleNamespaces, k)
	}
	for k := range newVisibleNamespaces {
		visibleNamespaces[k] = true
	}

	// Add nodes to table
	i := 1
	for _, data := range newState {
		table.SetCell(i, 0, tview.NewTableCell(data.Name).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))
		table.SetCell(i, 1, tview.NewTableCell(data.Status).
			SetTextColor(func() tcell.Color {
				if data.Status == "Ready" {
					return tcell.ColorGreen
				}
				return tcell.ColorRed
			}()).
			SetExpansion(1))
		table.SetCell(i, 2, tview.NewTableCell(data.Version).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))
		table.SetCell(i, 3, tview.NewTableCell(data.PodCount).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))
		table.SetCell(i, 4, tview.NewTableCell(data.Age).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))
		table.SetCell(i, 5, tview.NewTableCell(data.PodIndicators).
			SetExpansion(1).
			SetAlign(tview.AlignLeft))
		i++
	}

	return nil
}

func main() {
	fmt.Println("Starting application...")

	// Define namespace flags
	var namespaces arrayFlags
	flag.Var(&namespaces, "N", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.Var(&namespaces, "namespace", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.Parse()

	// Create maps for included and excluded namespaces
	includeNamespaces := make(map[string]bool)
	excludeNamespaces := make(map[string]bool)
	visibleNamespaces := make(map[string]bool) // Track all visible namespaces

	for _, ns := range namespaces {
		if strings.HasPrefix(ns, "-") {
			// Remove the "-" prefix and add to exclude list
			excludeNamespaces[strings.TrimPrefix(ns, "-")] = true
		} else {
			includeNamespaces[ns] = true
		}
	}

	// Create the application
	app := tview.NewApplication()

	// Create main table
	table := tview.NewTable().
		SetFixed(1, 0).   // Fix the header row
		SetBorders(false) // No cell borders

	// Create details table with scrolling support
	detailsTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false) // Enable vertical selection for scrolling

	// Create a box for details table
	detailsBox := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle("Node Details (Use mouse wheel or arrow keys to scroll)").
		SetBorderAttributes(tcell.AttrDim)

	// Enable table selection and set selection style
	table.SetSelectable(true, true).
		SetSelectedStyle(tcell.StyleDefault.
			Background(tcell.ColorNavy).
			Foreground(tcell.ColorWhite))

	// Add headers
	headers := []string{"Node Name", "Status", "Version", "PODS", "Age", "Pods"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}

	fmt.Println("Creating Kubernetes client...")

	// Use the current context from kubectl
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// Get a rest.Config from the kubeConfig
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to get client config: %v", err))
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	fmt.Println("Fetching nodes...")

	// Store nodes for lookup
	nodeMap := make(map[string]*corev1.Node)

	// Initial data load
	if err := updateNodeData(clientset, table, nodeMap, includeNamespaces, excludeNamespaces, visibleNamespaces); err != nil {
		panic(err)
	}

	// Set initial selection
	table.Select(1, 0)

	// Create a box to hold everything
	box := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetBorderAttributes(tcell.AttrDim)

	// Create a flex container for the table
	tableFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true)

	// Create a flex container with padding
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 1, false). // Top padding
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 1, false). // Left padding
			AddItem(box, 0, 1, true).
			AddItem(nil, 1, 1, false), // Right padding
			0, 1, true)

	// Create a flex container for details
	detailsFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 1, false). // Top padding
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 1, false). // Left padding
			AddItem(detailsBox, 0, 1, true).
			AddItem(nil, 1, 1, false), // Right padding
			0, 1, true)

	// Track if we're showing details view
	showingDetails := false

	// Spinner state
	spinnerChars := []rune{'-', '\\', '|', '/'}
	var spinnerIndex atomic.Int32
	var isRefreshing atomic.Bool

	// Set up auto-refresh ticker
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if !showingDetails { // Only update when not in details view
				isRefreshing.Store(true)
				app.QueueUpdateDraw(func() {}) // Force initial draw

				// Update data in background
				go func() {
					if err := updateNodeData(clientset, table, nodeMap, includeNamespaces, excludeNamespaces, visibleNamespaces); err != nil {
						// Log error but don't crash
						fmt.Printf("Error updating data: %v\n", err)
					}

					app.QueueUpdateDraw(func() {
						// Restore selection after update
						if table.GetRowCount() > 1 {
							table.Select(1, 0)
						}
						isRefreshing.Store(false)
					})
				}()
			}
		}
	}()
	defer ticker.Stop()

	// Spinner animation ticker
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	go func() {
		for range spinnerTicker.C {
			if isRefreshing.Load() {
				spinnerIndex.Add(1)
				app.QueueUpdateDraw(func() {})
			}
		}
	}()
	defer spinnerTicker.Stop()

	// Set the draw func to handle resizing
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Calculate the available space for the table
		tableWidth := width - 2   // Account for borders
		tableHeight := height - 2 // Account for borders

		// Convert visible namespaces map to sorted slice
		var nsSlice []string
		for ns := range visibleNamespaces {
			nsSlice = append(nsSlice, ns)
		}
		sort.Strings(nsSlice)

		// Draw the namespaces on the bottom border with space at start and end
		namespaceList := " " + strings.Join(nsSlice, ", ") + " "
		tview.Print(screen, namespaceList, x+1, y+height-1, tableWidth, tview.AlignLeft, tcell.ColorYellow)

		// Draw spinner in top right if refreshing
		if isRefreshing.Load() {
			spinnerChar := string(spinnerChars[int(spinnerIndex.Load())%len(spinnerChars)])
			tview.Print(screen, spinnerChar, x+width-2, y, 1, tview.AlignRight, tcell.ColorYellow)
		}

		// Update table dimensions and draw it
		table.SetRect(x+1, y+1, tableWidth, tableHeight)
		tableFlex.SetRect(x+1, y+1, tableWidth, tableHeight)
		tableFlex.Draw(screen)

		return x, y, width, height
	})

	// Add keyboard input handler
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle ESC key to close details view
		if event.Key() == tcell.KeyEscape {
			if showingDetails {
				showingDetails = false
				app.SetRoot(flex, true)
				app.SetFocus(table)
				return nil
			}
		}

		// Handle scrolling in details view
		if showingDetails {
			row, _ := detailsTable.GetSelection()
			switch event.Key() {
			case tcell.KeyUp:
				if row > 0 {
					detailsTable.Select(row-1, 0)
				}
				return nil
			case tcell.KeyDown:
				if row < detailsTable.GetRowCount()-1 {
					detailsTable.Select(row+1, 0)
				}
				return nil
			case tcell.KeyPgUp:
				newRow := row - 10
				if newRow < 0 {
					newRow = 0
				}
				detailsTable.Select(newRow, 0)
				return nil
			case tcell.KeyPgDn:
				newRow := row + 10
				if newRow >= detailsTable.GetRowCount() {
					newRow = detailsTable.GetRowCount() - 1
				}
				detailsTable.Select(newRow, 0)
				return nil
			case tcell.KeyHome:
				detailsTable.Select(0, 0)
				return nil
			case tcell.KeyEnd:
				detailsTable.Select(detailsTable.GetRowCount()-1, 0)
				return nil
			}
		}

		// Only process other keys if details view is not shown
		if !showingDetails {
			row, col := table.GetSelection()
			switch event.Key() {
			case tcell.KeyUp:
				if row > 1 { // Don't select header row
					table.Select(row-1, col)
				}
				return nil
			case tcell.KeyDown:
				if row < table.GetRowCount()-1 {
					table.Select(row+1, col)
				}
				return nil
			case tcell.KeyLeft:
				if col > 0 {
					table.Select(row, col-1)
				}
				return nil
			case tcell.KeyRight:
				if col < table.GetColumnCount()-1 {
					table.Select(row, col+1)
				}
				return nil
			case tcell.KeyEnter:
				// Get selected node name
				nodeName := table.GetCell(row, 0).Text
				if node, ok := nodeMap[nodeName]; ok {
					// Clear and setup details table
					detailsTable.Clear()

					// Basic Info
					row := 0
					detailsTable.SetCell(row, 0, tview.NewTableCell("Basic Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Name").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Creation Time").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.CreationTimestamp.Format(time.RFC3339)).SetTextColor(tcell.ColorWhite))
					row++

					// System Info
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("System Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Machine ID").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.MachineID).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("System UUID").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.SystemUUID).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Boot ID").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.BootID).SetTextColor(tcell.ColorWhite))
					row++

					// Version Info
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Version Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Kernel Version").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.KernelVersion).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("OS Image").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.OSImage).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Container Runtime").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.ContainerRuntimeVersion).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Architecture").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.Architecture).SetTextColor(tcell.ColorWhite))
					row++

					// Resource Info
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Resource Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("CPU").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.Capacity.Cpu().String()).SetTextColor(tcell.ColorWhite))
					row++
					detailsTable.SetCell(row, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorSkyblue))
					detailsTable.SetCell(row, 1, tview.NewTableCell(node.Status.Capacity.Memory().String()).SetTextColor(tcell.ColorWhite))
					row++

					// Labels and Annotations
					row++
					row = formatMapAsRows(detailsTable, row, "Labels", node.Labels)
					row++
					row = formatMapAsRows(detailsTable, row, "Annotations", node.Annotations)

					// Set initial selection for scrolling
					detailsTable.Select(0, 0)

					// Update details box
					detailsBox.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
						detailsTable.SetRect(x+1, y+1, width-2, height-2)
						detailsTable.Draw(screen)
						return x, y, width, height
					})

					showingDetails = true
					app.SetRoot(detailsFlex, true)
					app.SetFocus(detailsTable)
					return nil
				}
			}
		}
		return event
	})

	// Handle mouse wheel events for scrolling
	app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if showingDetails && action == tview.MouseScrollUp {
			row, _ := detailsTable.GetSelection()
			if row > 0 {
				detailsTable.Select(row-1, 0)
			}
			return nil, 0
		}
		if showingDetails && action == tview.MouseScrollDown {
			row, _ := detailsTable.GetSelection()
			if row < detailsTable.GetRowCount()-1 {
				detailsTable.Select(row+1, 0)
			}
			return nil, 0
		}
		return event, action
	})

	// Handle window resize
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		// Get new screen dimensions
		width, height := screen.Size()
		// Update flex container dimensions
		if !showingDetails {
			flex.SetRect(0, 0, width, height)
		} else {
			detailsFlex.SetRect(0, 0, width, height)
		}
		return false // Let the application draw normally
	})

	fmt.Println("Starting UI...")

	// Set input focus to the table
	app.SetFocus(table)

	// Run application
	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
