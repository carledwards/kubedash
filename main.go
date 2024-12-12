package main

import (
	"flag"
	"fmt"
	"k8s-nodes-example/cmd"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	fmt.Println("Starting application...")

	// Define namespace flags
	var namespaces []string
	flag.Var((*cmd.ArrayFlags)(&namespaces), "N", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.Var((*cmd.ArrayFlags)(&namespaces), "namespace", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
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

	fmt.Println("Creating Kubernetes client...")

	// Create Kubernetes client
	kubeClient, clusterName, err := cmd.NewKubeClient()
	if err != nil {
		panic(err)
	}

	// Create a box to hold everything
	box := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(fmt.Sprintf(" %s ", clusterName)). // Set cluster name as title with padding
		SetTitleAlign(tview.AlignCenter).           // Center the title
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

	fmt.Println("Fetching nodes...")

	// Initial data load
	nodeData, podsByNode, err := cmd.UpdateNodeData(kubeClient.Clientset, nodeView.GetNodeMap(), includeNamespaces, excludeNamespaces)
	if err != nil {
		panic(err)
	}

	// Update table with initial data
	updateTable(table, nodeData, podsByNode)

	// Set initial selection
	table.Select(1, 0)

	// Set up auto-refresh ticker
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if !showingDetails { // Only update when not in details view
				isRefreshing.Store(true)
				app.QueueUpdateDraw(func() {}) // Force initial draw

				// Store current selection
				currentRow, currentCol := table.GetSelection()

				// Update data in background
				go func() {
					nodeData, podsByNode, err := cmd.UpdateNodeData(kubeClient.Clientset, nodeView.GetNodeMap(), includeNamespaces, excludeNamespaces)
					if err != nil {
						// Log error but don't crash
						fmt.Printf("Error updating data: %v\n", err)
						return
					}

					app.QueueUpdateDraw(func() {
						updateTable(table, nodeData, podsByNode)
						// Restore previous selection if it's still valid
						if currentRow < table.GetRowCount() {
							table.Select(currentRow, currentCol)
						} else if table.GetRowCount() > 1 {
							// If the previous row no longer exists, select the last row
							table.Select(table.GetRowCount()-1, currentCol)
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

// updateTable updates the table with fresh node and pod data
func updateTable(table *tview.Table, nodeData map[string]cmd.NodeData, podsByNode map[string]map[string][]string) {
	// Clear existing data but preserve headers
	table.Clear()

	// Create headers
	headers := []string{"Node Name", "Status", "Version", "Age", "PODS"}

	// Get all namespaces from podsByNode
	namespaceSet := make(map[string]bool)
	for _, namespacePods := range podsByNode {
		for ns := range namespacePods {
			namespaceSet[ns] = true
		}
	}

	// Convert to sorted slice
	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// Add namespace headers
	headers = append(headers, namespaces...)

	// Set up headers
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}

	// Create a sorted slice of node names
	var nodeNames []string
	for name := range nodeData {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	// Add nodes to table in alphabetical order
	i := 1
	for _, nodeName := range nodeNames {
		data := nodeData[nodeName]
		// Basic node info
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
		table.SetCell(i, 3, tview.NewTableCell(data.Age).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))
		table.SetCell(i, 4, tview.NewTableCell(data.PodCount).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))

		// Add namespace columns
		for nsIdx, namespace := range namespaces {
			indicators := podsByNode[data.Name][namespace]
			cell := tview.NewTableCell(strings.Join(indicators, "")).
				SetExpansion(1).
				SetAlign(tview.AlignLeft)
			table.SetCell(i, 5+nsIdx, cell)
		}
		i++
	}
}
