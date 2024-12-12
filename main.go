package main

import (
	"context"
	"flag"
	"fmt"
	"k8s-nodes-example/cmd"
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

// updateNodeData updates the table with fresh node and pod data
func updateNodeData(clientset *kubernetes.Clientset, table *tview.Table, nodeMap map[string]*corev1.Node, includeNamespaces, excludeNamespaces, visibleNamespaces map[string]bool) error {
	// Store current state
	oldState := make(map[string]cmd.NodeData)
	for row := 1; row < table.GetRowCount(); row++ {
		name := table.GetCell(row, 0).Text
		oldState[name] = cmd.NodeData{
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
	newState := make(map[string]cmd.NodeData)
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
		age := cmd.FormatDuration(time.Since(node.CreationTimestamp.Time))

		// Store new state
		newState[node.Name] = cmd.NodeData{
			Name:          node.Name,
			Status:        status,
			Version:       node.Status.NodeInfo.KubeletVersion,
			PodCount:      podCountStr,
			Age:           age,
			PodIndicators: strings.Join(podIndicators, ""),
		}
	}

	// Check if there are any changes
	if !cmd.CompareNodes(oldState, newState) {
		return nil // No changes, don't update display
	}

	// Clear existing data but preserve headers
	table.Clear()

	// Re-add headers
	cmd.SetupNodeTable(table)

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
	nodeView := cmd.NewNodeView(includeNamespaces, excludeNamespaces)
	table := nodeView.GetTable()

	// Create details view
	detailsView := cmd.NewNodeDetailsView()

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

	// Track if we're showing details view
	showingDetails := false

	// Spinner state
	spinnerChars := []rune{'-', '\\', '|', '/'}
	var spinnerIndex atomic.Int32
	var isRefreshing atomic.Bool

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

	// Initial data load
	if err := updateNodeData(clientset, table, nodeView.GetNodeMap(), includeNamespaces, excludeNamespaces, nodeView.GetVisibleNamespaces()); err != nil {
		panic(err)
	}

	// Set initial selection
	table.Select(1, 0)

	// Set up auto-refresh ticker
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if !showingDetails { // Only update when not in details view
				isRefreshing.Store(true)
				app.QueueUpdateDraw(func() {}) // Force initial draw

				// Update data in background
				go func() {
					if err := updateNodeData(clientset, table, nodeView.GetNodeMap(), includeNamespaces, excludeNamespaces, nodeView.GetVisibleNamespaces()); err != nil {
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
		for ns := range nodeView.GetVisibleNamespaces() {
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
			row, _ := detailsView.GetTable().GetSelection()
			switch event.Key() {
			case tcell.KeyUp:
				if row > 0 {
					detailsView.GetTable().Select(row-1, 0)
				}
				return nil
			case tcell.KeyDown:
				if row < detailsView.GetTable().GetRowCount()-1 {
					detailsView.GetTable().Select(row+1, 0)
				}
				return nil
			case tcell.KeyPgUp:
				newRow := row - 10
				if newRow < 0 {
					newRow = 0
				}
				detailsView.GetTable().Select(newRow, 0)
				return nil
			case tcell.KeyPgDn:
				newRow := row + 10
				if newRow >= detailsView.GetTable().GetRowCount() {
					newRow = detailsView.GetTable().GetRowCount() - 1
				}
				detailsView.GetTable().Select(newRow, 0)
				return nil
			case tcell.KeyHome:
				detailsView.GetTable().Select(0, 0)
				return nil
			case tcell.KeyEnd:
				detailsView.GetTable().Select(detailsView.GetTable().GetRowCount()-1, 0)
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
				if node, ok := nodeView.GetNodeMap()[nodeName]; ok {
					detailsView.ShowNodeDetails(node)
					showingDetails = true
					app.SetRoot(detailsView.GetFlex(), true)
					app.SetFocus(detailsView.GetTable())
					return nil
				}
			}
		}
		return event
	})

	// Handle mouse wheel events for scrolling
	app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if showingDetails && action == tview.MouseScrollUp {
			row, _ := detailsView.GetTable().GetSelection()
			if row > 0 {
				detailsView.GetTable().Select(row-1, 0)
			}
			return nil, 0
		}
		if showingDetails && action == tview.MouseScrollDown {
			row, _ := detailsView.GetTable().GetSelection()
			if row < detailsView.GetTable().GetRowCount()-1 {
				detailsView.GetTable().Select(row+1, 0)
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
			detailsView.GetFlex().SetRect(0, 0, width, height)
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
