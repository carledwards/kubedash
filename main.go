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

	// Define flags
	var namespaces []string
	var useMockData bool
	flag.Var((*cmd.ArrayFlags)(&namespaces), "N", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.Var((*cmd.ArrayFlags)(&namespaces), "namespace", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.BoolVar(&useMockData, "mock-k8s-data", false, "Use mock Kubernetes data instead of real cluster")
	flag.Parse()

	// Create maps for included and excluded namespaces
	includeNamespaces := make(map[string]bool)
	excludeNamespaces := make(map[string]bool)

	for _, ns := range namespaces {
		if strings.HasPrefix(ns, "-") {
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

	// Create changelog view
	changeLogView := cmd.NewChangeLogView()
	changeLogTable := changeLogView.GetTable()

	// Create state cache for tracking changes
	stateCache := cmd.NewStateCache()

	fmt.Println("Creating data provider...")

	// Create data provider based on flag
	var dataProvider cmd.K8sDataProvider
	var err error
	if useMockData {
		dataProvider = cmd.NewMockK8sDataProvider()
	} else {
		dataProvider, err = cmd.NewRealK8sDataProvider()
		if err != nil {
			panic(err)
		}
	}

	// Create a box to hold everything
	box := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(fmt.Sprintf(" %s ", dataProvider.GetClusterName())).
		SetTitleAlign(tview.AlignCenter).
		SetBorderAttributes(tcell.AttrDim)

	// Create a flex container for the table and changelog
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add items to mainFlex with proper focus handling
	mainFlex.AddItem(table, 0, 2, true)
	mainFlex.AddItem(changeLogView.GetFlex(), 0, 1, false)

	// Create a flex container with padding
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 1, false).
			AddItem(box, 0, 1, true).
			AddItem(nil, 1, 1, false),
			0, 1, true)

	// Track if we're showing details view
	showingDetails := false

	// Spinner state
	spinnerChars := []rune{'-', '\\', '|', '/'}
	var spinnerIndex atomic.Int32
	var isRefreshing atomic.Bool

	fmt.Println("Fetching initial data...")

	// Initial data load
	nodeData, podsByNode, err := dataProvider.UpdateNodeData(includeNamespaces, excludeNamespaces)
	if err != nil {
		panic(err)
	}

	// Update nodeView's map with the provider's map
	for k, v := range dataProvider.GetNodeMap() {
		nodeView.GetNodeMap()[k] = v
	}

	// Store initial state in cache
	for nodeName, data := range nodeData {
		stateCache.Put(nodeName, cmd.ResourceState{
			Data:      data,
			Timestamp: time.Now(),
		})
	}

	// Update table with initial data
	updateTable(table, nodeData, podsByNode)

	// Set initial selection
	table.Select(1, 0)

	// Set up auto-refresh ticker
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if !showingDetails {
				isRefreshing.Store(true)
				app.QueueUpdateDraw(func() {})

				currentRow, currentCol := table.GetSelection()

				go func() {
					newNodeData, newPodsByNode, err := dataProvider.UpdateNodeData(includeNamespaces, excludeNamespaces)
					if err != nil {
						fmt.Printf("Error updating data: %v\n", err)
						return
					}

					app.QueueUpdateDraw(func() {
						// Check for changes and update changelog
						for nodeName, newData := range newNodeData {
							changes := stateCache.Compare(nodeName, cmd.ResourceState{
								Data:      newData,
								Timestamp: time.Now(),
							})
							for _, change := range changes {
								changeLogView.AddChange(change)
							}
						}

						// Check for removed nodes
						for nodeName := range nodeView.GetNodeMap() {
							if _, exists := newNodeData[nodeName]; !exists {
								changes := stateCache.Compare(nodeName, cmd.ResourceState{
									Data:      nil,
									Timestamp: time.Now(),
								})
								for _, change := range changes {
									changeLogView.AddChange(change)
								}
							}
						}

						// Update nodeView's map
						for k := range nodeView.GetNodeMap() {
							delete(nodeView.GetNodeMap(), k)
						}
						for k, v := range dataProvider.GetNodeMap() {
							nodeView.GetNodeMap()[k] = v
						}

						updateTable(table, newNodeData, newPodsByNode)
						if currentRow < table.GetRowCount() {
							table.Select(currentRow, currentCol)
						} else if table.GetRowCount() > 1 {
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
		if isRefreshing.Load() {
			spinnerChar := string(spinnerChars[int(spinnerIndex.Load())%len(spinnerChars)])
			tview.Print(screen, spinnerChar, x+width-2, y, 1, tview.AlignRight, tcell.ColorYellow)
		}

		mainFlex.SetRect(x+1, y+1, width-2, height-2)
		mainFlex.Draw(screen)

		return x, y, width, height
	})

	// Track which component has focus
	focusIndex := 0
	components := []tview.Primitive{table, changeLogTable}

	// Add keyboard input handler
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			if showingDetails {
				showingDetails = false
				app.SetRoot(flex, true)
				app.SetFocus(table)
				return nil
			}
		}

		// Handle Tab key for switching focus between main table and changelog
		if !showingDetails && event.Key() == tcell.KeyTab {
			focusIndex = (focusIndex + 1) % len(components)
			app.SetFocus(components[focusIndex])
			return nil
		}

		// If changelog has focus and 'c' is pressed, clear it
		if !showingDetails && app.GetFocus() == changeLogTable && event.Rune() == 'c' {
			changeLogView.Clear()
			return nil
		}

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

		if !showingDetails {
			row, col := table.GetSelection()
			switch event.Key() {
			case tcell.KeyUp:
				if row > 1 {
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
		width, height := screen.Size()
		if !showingDetails {
			flex.SetRect(0, 0, width, height)
		} else {
			detailsView.GetFlex().SetRect(0, 0, width, height)
		}
		return false
	})

	fmt.Println("Starting UI...")

	app.SetFocus(table)

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

// updateTable updates the table with fresh node and pod data
func updateTable(table *tview.Table, nodeData map[string]cmd.NodeData, podsByNode map[string]map[string][]string) {
	table.Clear()

	headers := []string{"Node Name", "Status", "Version", "Age", "PODS"}

	namespaceSet := make(map[string]bool)
	for _, namespacePods := range podsByNode {
		for ns := range namespacePods {
			namespaceSet[ns] = true
		}
	}

	var namespaces []string
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	headers = append(headers, namespaces...)

	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		table.SetCell(0, i, cell)
	}

	var nodeNames []string
	for name := range nodeData {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	i := 1
	for _, nodeName := range nodeNames {
		data := nodeData[nodeName]
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
