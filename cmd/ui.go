package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UI manages all UI components and interactions
type UI struct {
	app            *tview.Application
	nodeView       *NodeView
	detailsView    *NodeDetailsView
	podDetailsView *PodDetailsView
	logView        *LogView
	changeLogView  *ChangeLogView
	mainApp        *App
	focusIndex     int
	components     []tview.Primitive
	mainFlex       *tview.Flex
	pages          *tview.Pages
	errorModal     *tview.Modal
	helpModal      *tview.Modal
	mainBox        *tview.Box
}

// NewUI creates a new UI instance
func NewUI(mainApp *App) *UI {
	ui := &UI{
		app:     tview.NewApplication(),
		mainApp: mainApp,
		pages:   tview.NewPages(),
	}
	return ui
}

// ShowErrorMessage displays an error message modal
func (ui *UI) ShowErrorMessage() {
	if ui.errorModal == nil {
		ui.errorModal = tview.NewModal().
			SetText(ErrorDialogText)
	}
	ui.pages.AddPage("error", ui.errorModal, false, true)
}

// DismissErrorMessage removes the error message modal
func (ui *UI) DismissErrorMessage() {
	ui.pages.RemovePage("error")
}

// ShowHelpModal displays the keyboard shortcuts help
func (ui *UI) ShowHelpModal() {
	if ui.helpModal == nil {
		ui.helpModal = tview.NewModal().
			SetText(HelpDialogText).
			SetBackgroundColor(tcell.ColorDimGray)
	}
	ui.pages.AddPage("help", ui.helpModal, false, true)
}

// DismissHelpModal removes the help modal
func (ui *UI) DismissHelpModal() {
	ui.pages.RemovePage("help")
}

// Setup initializes all UI components
func (ui *UI) Setup() error {
	// Create main table
	ui.nodeView = NewNodeView(ui.mainApp.config.IncludeNamespaces, ui.mainApp.config.ExcludeNamespaces)
	table := ui.nodeView.GetTable()

	// Create details views
	ui.detailsView = NewNodeDetailsView()
	ui.podDetailsView = NewPodDetailsView()
	ui.logView = NewLogView()
	ui.logView.SetApplication(ui.app)
	ui.logView.SetMainApp(ui.mainApp) // Set the main app reference

	// Create changelog view
	ui.changeLogView = NewChangeLogView(ui.mainApp.config.LogFilePath)
	changeLogTable := ui.changeLogView.GetTable()

	// Track focusable components
	ui.components = []tview.Primitive{table, changeLogTable}

	// Create a box to hold everything
	ui.mainBox = tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(fmt.Sprintf("  %s  ", ui.mainApp.GetProvider().GetClusterName())).
		SetTitleAlign(tview.AlignCenter).
		SetBorderAttributes(tcell.AttrDim)

	// Set the application and box in the changelog view for flashing effect
	ui.changeLogView.SetApplication(ui.app)
	ui.changeLogView.SetBox(ui.mainBox)

	// Create a flex container for the table and changelog
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add items to mainFlex with proper focus handling
	mainFlex.AddItem(table, 0, 2, true)
	mainFlex.AddItem(ui.changeLogView.GetFlex(), 0, 1, false)

	// Create a flex container without top padding
	ui.mainFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 1, false).
			AddItem(ui.mainBox, 0, 1, true).
			AddItem(nil, 1, 1, false),
			0, 1, true)

	// Add main UI to pages
	ui.pages.AddPage("main", ui.mainFlex, true, true)

	// Set up input handling
	ui.setupKeyboardHandling()
	ui.setupMouseHandling()

	// Set the draw func to handle resizing
	ui.mainBox.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		mainFlex.SetRect(x+1, y+1, width-2, height-2)
		mainFlex.Draw(screen)
		return x, y, width, height
	})

	// Handle window resize
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		if !ui.mainApp.IsShowingDetails() && !ui.mainApp.IsShowingPods() {
			ui.pages.SetRect(0, 0, width, height)
		} else if ui.mainApp.IsShowingPods() {
			ui.podDetailsView.GetFlex().SetRect(0, 0, width, height)
		} else {
			ui.detailsView.GetFlex().SetRect(0, 0, width, height)
		}
		return false
	})

	ui.app.SetFocus(table)
	ui.app.SetRoot(ui.pages, true).EnableMouse(true)

	return nil
}

// hasActiveModal checks if any modal is currently displayed
func (ui *UI) hasActiveModal() bool {
	return ui.pages.HasPage("error") || ui.pages.HasPage("help")
}

// setupKeyboardHandling sets up keyboard input handling
func (ui *UI) setupKeyboardHandling() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// If error modal is active, ignore all keyboard input
		if ui.pages.HasPage("error") {
			return nil
		}

		// If help modal is active, only handle Esc key
		if ui.pages.HasPage("help") {
			if event.Key() == tcell.KeyEscape {
				ui.DismissHelpModal()
			}
			return nil
		}

		// Handle global '?' key for help when no modal is active
		if !ui.hasActiveModal() && event.Rune() == KeyHelp {
			ui.ShowHelpModal()
			return nil
		}

		// If showing pod details, handle its specific keys
		if ui.mainApp.IsShowingPods() {
			if event.Key() == tcell.KeyEscape {
				ui.mainApp.SetShowingPods(false)
				ui.app.SetRoot(ui.pages, true)
				ui.app.SetFocus(ui.nodeView.GetTable())
				return nil
			}
			return ui.handlePodDetailsViewKeys(event)
		}

		// If showing node details view, handle its specific keys
		if ui.mainApp.IsShowingDetails() {
			if event.Key() == tcell.KeyEscape {
				ui.mainApp.SetShowingDetails(false)
				ui.app.SetRoot(ui.pages, true)
				ui.app.SetFocus(ui.nodeView.GetTable())
				return nil
			}
			return ui.handleDetailsViewKeys(event)
		}

		// Handle other global keys when no modal is active
		if !ui.hasActiveModal() {
			switch event.Rune() {
			case KeyClearHistory:
				ui.changeLogView.Clear()
				return nil
			case KeyRefresh:
				ui.mainApp.TriggerRefresh()
				return nil
			}

			// Handle Tab key
			if event.Key() == tcell.KeyTab {
				ui.focusIndex = (ui.focusIndex + 1) % len(ui.components)
				ui.app.SetFocus(ui.components[ui.focusIndex])
				return nil
			}

			return ui.handleMainViewKeys(event)
		}

		return nil
	})
}

// setupMouseHandling sets up mouse input handling
func (ui *UI) setupMouseHandling() {
	ui.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		// If any modal is active, ignore mouse input
		if ui.hasActiveModal() {
			return nil, 0
		}

		if (ui.mainApp.IsShowingPods() || ui.mainApp.IsShowingDetails()) && action == tview.MouseScrollUp {
			row, _ := ui.getCurrentDetailsTable().GetSelection()
			if row > 0 {
				ui.getCurrentDetailsTable().Select(row-1, 0)
			}
			return nil, 0
		}
		if (ui.mainApp.IsShowingPods() || ui.mainApp.IsShowingDetails()) && action == tview.MouseScrollDown {
			row, _ := ui.getCurrentDetailsTable().GetSelection()
			if row < ui.getCurrentDetailsTable().GetRowCount()-1 {
				ui.getCurrentDetailsTable().Select(row+1, 0)
			}
			return nil, 0
		}
		return event, action
	})
}

// getCurrentDetailsTable returns the currently active details table
func (ui *UI) getCurrentDetailsTable() *tview.Table {
	if ui.mainApp.IsShowingPods() {
		return ui.podDetailsView.GetTable()
	}
	return ui.detailsView.GetTable()
}

// handlePodDetailsViewKeys handles keyboard input for the pod details view
func (ui *UI) handlePodDetailsViewKeys(event *tcell.EventKey) *tcell.EventKey {
	row, _ := ui.podDetailsView.GetTable().GetSelection()
	switch event.Key() {
	case tcell.KeyEnter:
		if row > 0 { // Skip header row
			podName := ui.podDetailsView.GetTable().GetCell(row, 0).Text
			if podInfo, ok := ui.podDetailsView.GetPodInfo(podName); ok {
				// Set up log view with proper navigation
				ui.logView.SetPreviousApp(ui.podDetailsView.GetFlex())
				// Store the current table and selection for restoration
				ui.logView.SetPreviousSelection(ui.podDetailsView.GetTable(), row)
				ui.logView.ShowPodLogs(ui.mainApp.GetProvider().(*RealK8sDataProvider).client, &podInfo)
				ui.app.SetRoot(ui.logView.GetFlex(), true)
			}
		}
		return nil
	case tcell.KeyUp:
		if row > 0 {
			ui.podDetailsView.GetTable().Select(row-1, 0)
		}
		return nil
	case tcell.KeyDown:
		if row < ui.podDetailsView.GetTable().GetRowCount()-1 {
			ui.podDetailsView.GetTable().Select(row+1, 0)
		}
		return nil
	case tcell.KeyPgUp:
		newRow := row - 10
		if newRow < 0 {
			newRow = 0
		}
		ui.podDetailsView.GetTable().Select(newRow, 0)
		return nil
	case tcell.KeyPgDn:
		newRow := row + 10
		if newRow >= ui.podDetailsView.GetTable().GetRowCount() {
			newRow = ui.podDetailsView.GetTable().GetRowCount() - 1
		}
		ui.podDetailsView.GetTable().Select(newRow, 0)
		return nil
	case tcell.KeyHome:
		ui.podDetailsView.GetTable().Select(0, 0)
		return nil
	case tcell.KeyEnd:
		ui.podDetailsView.GetTable().Select(ui.podDetailsView.GetTable().GetRowCount()-1, 0)
		return nil
	}
	return event
}

// handleDetailsViewKeys handles keyboard input for the details view
func (ui *UI) handleDetailsViewKeys(event *tcell.EventKey) *tcell.EventKey {
	row, _ := ui.detailsView.GetTable().GetSelection()
	switch event.Key() {
	case tcell.KeyUp:
		if row > 0 {
			ui.detailsView.GetTable().Select(row-1, 0)
		}
		return nil
	case tcell.KeyDown:
		if row < ui.detailsView.GetTable().GetRowCount()-1 {
			ui.detailsView.GetTable().Select(row+1, 0)
		}
		return nil
	case tcell.KeyPgUp:
		newRow := row - 10
		if newRow < 0 {
			newRow = 0
		}
		ui.detailsView.GetTable().Select(newRow, 0)
		return nil
	case tcell.KeyPgDn:
		newRow := row + 10
		if newRow >= ui.detailsView.GetTable().GetRowCount() {
			newRow = ui.detailsView.GetTable().GetRowCount() - 1
		}
		ui.detailsView.GetTable().Select(newRow, 0)
		return nil
	case tcell.KeyHome:
		ui.detailsView.GetTable().Select(0, 0)
		return nil
	case tcell.KeyEnd:
		ui.detailsView.GetTable().Select(ui.detailsView.GetTable().GetRowCount()-1, 0)
		return nil
	}
	return event
}

// handleMainViewKeys handles keyboard input for the main view
func (ui *UI) handleMainViewKeys(event *tcell.EventKey) *tcell.EventKey {
	table := ui.nodeView.GetTable()
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
		if col <= 4 { // Node columns
			if node, ok := ui.nodeView.GetNodeMap()[nodeName]; ok {
				ui.detailsView.ShowNodeDetails(node)
				ui.mainApp.SetShowingDetails(true)
				ui.app.SetRoot(ui.detailsView.GetFlex(), true)
				ui.app.SetFocus(ui.detailsView.GetTable())
				return nil
			}
		} else { // Pod columns
			namespace := table.GetCell(0, col).Text
			if podsByNode, err := ui.mainApp.GetProvider().GetPodsByNode(ui.mainApp.config.IncludeNamespaces, ui.mainApp.config.ExcludeNamespaces); err == nil {
				if nodePods, ok := podsByNode[nodeName]; ok {
					// Filter pods by namespace
					namespacePods := make(map[string]PodInfo)
					for podName, podInfo := range nodePods {
						if podInfo.Namespace == namespace {
							namespacePods[podName] = podInfo
						}
					}
					ui.podDetailsView.ShowPodDetails(nodeName, namespace, namespacePods)
					ui.mainApp.SetShowingPods(true)
					ui.app.SetRoot(ui.podDetailsView.GetFlex(), true)
					ui.app.SetFocus(ui.podDetailsView.GetTable())
					return nil
				}
			}
		}
	}
	return event
}

// UpdateTable updates the table with fresh node and pod data
func (ui *UI) UpdateTable(nodeData map[string]NodeData, podsByNode map[string]map[string][]string) {
	table := ui.nodeView.GetTable()
	currentRow, currentCol := table.GetSelection()

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

	// Set up header row
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

	var nodeNames []string
	for name := range nodeData {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	i := 1
	for _, nodeName := range nodeNames {
		data := nodeData[nodeName]

		// Node Name column
		table.SetCell(i, 0, tview.NewTableCell(data.Name).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))

		// Status column
		table.SetCell(i, 1, tview.NewTableCell(data.Status).
			SetTextColor(func() tcell.Color {
				if data.Status == NodeStatusReady {
					return tcell.ColorGreen
				}
				return tcell.ColorRed
			}()).
			SetExpansion(1))

		// Version column
		table.SetCell(i, 2, tview.NewTableCell(data.Version).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1))

		// Age column
		table.SetCell(i, 3, tview.NewTableCell(data.Age).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1).
			SetAlign(tview.AlignRight))

		// PODS column
		table.SetCell(i, 4, tview.NewTableCell(data.PodCount).
			SetTextColor(tcell.ColorSkyblue).
			SetExpansion(1).
			SetAlign(tview.AlignRight))

		// Namespace columns with pod indicators
		for nsIdx, namespace := range namespaces {
			indicators := podsByNode[data.Name][namespace]
			cell := tview.NewTableCell(strings.Join(indicators, "")).
				SetExpansion(1).
				SetAlign(tview.AlignLeft)
			table.SetCell(i, 5+nsIdx, cell)
		}
		i++
	}

	// Restore selection
	if currentRow < table.GetRowCount() {
		table.Select(currentRow, currentCol)
	} else if table.GetRowCount() > 1 {
		table.Select(table.GetRowCount()-1, currentCol)
	}
}
