package cmd

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ChangeLogView represents the view for displaying change events
type ChangeLogView struct {
	table *tview.Table
	box   *tview.Box
	flex  *tview.Flex
}

// NewChangeLogView creates a new ChangeLogView instance
func NewChangeLogView() *ChangeLogView {
	changeTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	// Set up headers
	headers := []string{"Time", "Resource", "Name", "Change", "Field", "Old Value", "New Value"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		changeTable.SetCell(0, i, cell)
	}

	changeBox := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle("Change Log (Most Recent First)").
		SetBorderAttributes(tcell.AttrDim)

	// Create a flex container
	changeFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(changeBox, 0, 1, true)

	return &ChangeLogView{
		table: changeTable,
		box:   changeBox,
		flex:  changeFlex,
	}
}

// GetFlex returns the flex container
func (cv *ChangeLogView) GetFlex() *tview.Flex {
	return cv.flex
}

// AddChange adds a new change event to the log
func (cv *ChangeLogView) AddChange(change ChangeEvent) {
	// Shift existing rows down (except header)
	rowCount := cv.table.GetRowCount()
	if rowCount > 1 {
		for row := rowCount - 1; row >= 1; row-- {
			for col := 0; col < 7; col++ {
				cell := cv.table.GetCell(row, col)
				if cell != nil {
					cv.table.SetCell(row+1, col, cell)
				}
			}
		}
	}

	// Format the timestamp
	timeStr := change.Timestamp.Format("15:04:05")

	// Add the new row at position 1 (right after header)
	cv.table.SetCell(1, 0, tview.NewTableCell(timeStr).SetTextColor(tcell.ColorWhite))
	cv.table.SetCell(1, 1, tview.NewTableCell(change.ResourceType).SetTextColor(tcell.ColorYellow))
	cv.table.SetCell(1, 2, tview.NewTableCell(change.ResourceName).SetTextColor(tcell.ColorAqua))

	// Set change type with appropriate color
	changeCell := tview.NewTableCell(change.ChangeType)
	switch change.ChangeType {
	case "Added":
		changeCell.SetTextColor(tcell.ColorGreen)
	case "Removed":
		changeCell.SetTextColor(tcell.ColorRed)
	case "Modified":
		changeCell.SetTextColor(tcell.ColorYellow)
	}
	cv.table.SetCell(1, 3, changeCell)

	// Field name
	cv.table.SetCell(1, 4, tview.NewTableCell(change.Field).SetTextColor(tcell.ColorSkyblue))

	// Old and new values with better formatting
	oldValue := formatValue(change.OldValue)
	newValue := formatValue(change.NewValue)
	cv.table.SetCell(1, 5, tview.NewTableCell(oldValue).SetTextColor(tcell.ColorGray))
	cv.table.SetCell(1, 6, tview.NewTableCell(newValue).SetTextColor(tcell.ColorWhite))

	// Update box draw function
	cv.box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		cv.table.SetRect(x+1, y+1, width-2, height-2)
		cv.table.Draw(screen)
		return x, y, width, height
	})
}

// formatValue formats a value for display in the changelog
func formatValue(value interface{}) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%v", value)
}

// Clear clears all entries from the change log
func (cv *ChangeLogView) Clear() {
	cv.table.Clear()

	// Restore headers
	headers := []string{"Time", "Resource", "Name", "Change", "Field", "Old Value", "New Value"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		cv.table.SetCell(0, i, cell)
	}
}
