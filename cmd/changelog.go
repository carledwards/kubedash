package cmd

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ChangeLogView represents the view for displaying change events
type ChangeLogView struct {
	table *tview.Table
	flex  *tview.Flex
}

// NewChangeLogView creates a new ChangeLogView instance
func NewChangeLogView() *ChangeLogView {
	changeTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).                                      // Make sure table is selectable
		SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorNavy)) // Add visual feedback for focus

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

	// Create a flex container
	changeFlex := tview.NewFlex().
		SetDirection(tview.FlexRow)

	// Set border and title on the table itself
	changeTable.SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(" Change Log ").
		SetBorderAttributes(tcell.AttrDim)

	// Add the table to the flex with focus enabled
	changeFlex.AddItem(changeTable, 0, 1, true)

	cv := &ChangeLogView{
		table: changeTable,
		flex:  changeFlex,
	}

	// Ensure the table starts with a selection
	changeTable.Select(0, 0)

	return cv
}

// GetFlex returns the flex container
func (cv *ChangeLogView) GetFlex() *tview.Flex {
	return cv.flex
}

// GetTable returns the underlying table primitive
func (cv *ChangeLogView) GetTable() *tview.Table {
	return cv.table
}

// AddChange adds a new change event to the log
func (cv *ChangeLogView) AddChange(change ChangeEvent) {
	// Format the row data
	cells := []*tview.TableCell{
		tview.NewTableCell(change.Timestamp.Format("15:04:05")).SetTextColor(tcell.ColorWhite),
		tview.NewTableCell(change.ResourceType).SetTextColor(tcell.ColorYellow),
		tview.NewTableCell(change.ResourceName).SetTextColor(tcell.ColorAqua),
		tview.NewTableCell(change.ChangeType).SetTextColor(func() tcell.Color {
			switch change.ChangeType {
			case "Added":
				return tcell.ColorGreen
			case "Removed":
				return tcell.ColorRed
			case "Modified":
				return tcell.ColorYellow
			default:
				return tcell.ColorWhite
			}
		}()),
		tview.NewTableCell(change.Field).SetTextColor(tcell.ColorSkyblue),
		tview.NewTableCell(formatValue(change.OldValue)).SetTextColor(tcell.ColorGray),
		tview.NewTableCell(formatValue(change.NewValue)).SetTextColor(tcell.ColorWhite),
	}

	// Add the row to the table
	cv.addRowReverseWithTruncate(cells, 20)

	// Optional: Ensure focus stays at the top
	cv.table.Select(1, 0)
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

	// Ensure selection is maintained after clearing
	cv.table.Select(0, 0)
}

func (cv *ChangeLogView) addRowReverseWithTruncate(cells []*tview.TableCell, maxRows int) {
	rowCount := cv.table.GetRowCount()
	for row := rowCount - 1; row > 0; row-- { // Shift existing rows down
		for col := 0; col < cv.table.GetColumnCount(); col++ {
			cell := cv.table.GetCell(row, col)
			if cell != nil {
				cv.table.SetCell(row+1, col, cell)
			}
		}
	}

	// Insert the new row at the top
	for col, cell := range cells {
		cv.table.SetCell(1, col, cell)
	}

	// Truncate rows if exceeding maxRows
	if rowCount >= maxRows {
		cv.table.RemoveRow(rowCount)
	}

	cv.table.ScrollToBeginning()
}
