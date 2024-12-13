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
	app           *tview.Application
	nodeView      *NodeView
	detailsView   *NodeDetailsView
	changeLogView *ChangeLogView
	mainApp       *App
	focusIndex    int
	components    []tview.Primitive
	mainFlex      *tview.Flex
}

// NewUI creates a new UI instance
func NewUI(mainApp *App) *UI {
	ui := &UI{
		app:     tview.NewApplication(),
		mainApp: mainApp,
	}
	return ui
}

// Setup initializes all UI components
func (ui *UI) Setup() error {
	// Create main table
	ui.nodeView = NewNodeView(ui.mainApp.config.IncludeNamespaces, ui.mainApp.config.ExcludeNamespaces)
	table := ui.nodeView.GetTable()

	// Create details view
	ui.detailsView = NewNodeDetailsView()

	// Create changelog view
	ui.changeLogView = NewChangeLogView(ui.mainApp.config.LogFilePath)
	changeLogTable := ui.changeLogView.GetTable()

	// Track focusable components
	ui.components = []tview.Primitive{table, changeLogTable}

	// Create a box to hold everything
	box := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(fmt.Sprintf(" %s ", ui.mainApp.GetProvider().GetClusterName())).
		SetTitleAlign(tview.AlignCenter).
		SetBorderAttributes(tcell.AttrDim)

	// Set the application and box in the changelog view for flashing effect
	ui.changeLogView.SetApplication(ui.app)
	ui.changeLogView.SetBox(box)

	// Create a flex container for the table and changelog
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Add items to mainFlex with proper focus handling
	mainFlex.AddItem(table, 0, 2, true)
	mainFlex.AddItem(ui.changeLogView.GetFlex(), 0, 1, false)

	// Create a flex container with padding
	ui.mainFlex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(nil, 1, 1, false).
			AddItem(box, 0, 1, true).
			AddItem(nil, 1, 1, false),
			0, 1, true)

	// Set up input handling
	ui.setupKeyboardHandling()
	ui.setupMouseHandling()

	// Set the draw func to handle resizing
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		if ui.mainApp.IsRefreshing() {
			spinnerChar := string(ui.mainApp.GetSpinnerChar())
			tview.Print(screen, spinnerChar, x+width-2, y, 1, tview.AlignRight, tcell.ColorYellow)
		}

		mainFlex.SetRect(x+1, y+1, width-2, height-2)
		mainFlex.Draw(screen)

		return x, y, width, height
	})

	// Handle window resize
	ui.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		width, height := screen.Size()
		if !ui.mainApp.IsShowingDetails() {
			ui.mainFlex.SetRect(0, 0, width, height)
		} else {
			ui.detailsView.GetFlex().SetRect(0, 0, width, height)
		}
		return false
	})

	ui.app.SetFocus(table)
	ui.app.SetRoot(ui.mainFlex, true).EnableMouse(true)

	return nil
}

// setupKeyboardHandling sets up keyboard input handling
func (ui *UI) setupKeyboardHandling() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Handle global 'c' key for clearing changelog
		if !ui.mainApp.IsShowingDetails() && event.Rune() == 'c' {
			ui.changeLogView.Clear()
			return nil
		}

		// Handle global 'r' key for refreshing data
		if !ui.mainApp.IsShowingDetails() && event.Rune() == 'r' {
			if err := ui.mainApp.refreshData(); err != nil {
				fmt.Printf("Error refreshing data: %v\n", err)
			}
			return nil
		}

		if event.Key() == tcell.KeyEscape {
			if ui.mainApp.IsShowingDetails() {
				ui.mainApp.SetShowingDetails(false)
				ui.app.SetRoot(ui.mainFlex, true)
				ui.app.SetFocus(ui.nodeView.GetTable())
				return nil
			}
		}

		// Handle Tab key for switching focus between main table and changelog
		if !ui.mainApp.IsShowingDetails() && event.Key() == tcell.KeyTab {
			ui.focusIndex = (ui.focusIndex + 1) % len(ui.components)
			ui.app.SetFocus(ui.components[ui.focusIndex])
			return nil
		}

		if ui.mainApp.IsShowingDetails() {
			return ui.handleDetailsViewKeys(event)
		}

		return ui.handleMainViewKeys(event)
	})
}

// setupMouseHandling sets up mouse input handling
func (ui *UI) setupMouseHandling() {
	ui.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		if ui.mainApp.IsShowingDetails() && action == tview.MouseScrollUp {
			row, _ := ui.detailsView.GetTable().GetSelection()
			if row > 0 {
				ui.detailsView.GetTable().Select(row-1, 0)
			}
			return nil, 0
		}
		if ui.mainApp.IsShowingDetails() && action == tview.MouseScrollDown {
			row, _ := ui.detailsView.GetTable().GetSelection()
			if row < ui.detailsView.GetTable().GetRowCount()-1 {
				ui.detailsView.GetTable().Select(row+1, 0)
			}
			return nil, 0
		}
		return event, action
	})
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
		if node, ok := ui.nodeView.GetNodeMap()[nodeName]; ok {
			ui.detailsView.ShowNodeDetails(node)
			ui.mainApp.SetShowingDetails(true)
			ui.app.SetRoot(ui.detailsView.GetFlex(), true)
			ui.app.SetFocus(ui.detailsView.GetTable())
			return nil
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
