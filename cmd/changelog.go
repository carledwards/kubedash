package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ChangeLogView represents the view for displaying change events
type ChangeLogView struct {
	table   *tview.Table
	flex    *tview.Flex
	app     *tview.Application
	box     *tview.Box
	logFile *os.File
}

// NewChangeLogView creates a new ChangeLogView instance
func NewChangeLogView(logFilePath string) *ChangeLogView {
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

	var logFile *os.File
	if logFilePath != "" {
		var err error
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("Error opening log file: %v\n", err)
		}
	}

	// Set border and title on the table itself
	title := " Change Log "
	if logFile != nil {
		title = fmt.Sprintf(" Change Log [%s] ", filepath.Base(logFilePath))
	}

	changeTable.SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle(title).
		SetBorderAttributes(tcell.AttrDim)

	// Add the table to the flex with focus enabled
	changeFlex.AddItem(changeTable, 0, 1, true)

	cv := &ChangeLogView{
		table:   changeTable,
		flex:    changeFlex,
		logFile: logFile,
	}

	// Ensure the table starts with a selection
	changeTable.Select(0, 0)

	return cv
}

// Close closes the log file if it's open
func (cv *ChangeLogView) Close() {
	if cv.logFile != nil {
		cv.logFile.Close()
	}
}

// SetApplication sets the tview application instance
func (cv *ChangeLogView) SetApplication(app *tview.Application) {
	cv.app = app
}

// SetBox sets the main box instance
func (cv *ChangeLogView) SetBox(box *tview.Box) {
	cv.box = box
}

// GetFlex returns the flex container
func (cv *ChangeLogView) GetFlex() *tview.Flex {
	return cv.flex
}

// GetTable returns the underlying table primitive
func (cv *ChangeLogView) GetTable() *tview.Table {
	return cv.table
}

// flashTitle creates a flashing effect for the title
func (cv *ChangeLogView) flashTitle() {
	if cv.app == nil || cv.box == nil {
		return
	}

	originalColor := tcell.ColorGray
	flashColor := tcell.ColorYellow

	// Flash 3 times
	go func() {
		for i := 0; i < 3; i++ {
			cv.app.QueueUpdateDraw(func() {
				cv.box.SetBorderColor(flashColor)
			})
			time.Sleep(200 * time.Millisecond)

			cv.app.QueueUpdateDraw(func() {
				cv.box.SetBorderColor(originalColor)
			})
			time.Sleep(200 * time.Millisecond)
		}
	}()
}

// AddChange adds a new change event to the log
func (cv *ChangeLogView) AddChange(change ChangeEvent) {
	// Format the row data
	cells := []*tview.TableCell{
		tview.NewTableCell(change.Timestamp.Format("2006-01-02 15:04:05")).SetTextColor(tcell.ColorWhite),
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

	// Write to log file if enabled
	if cv.logFile != nil {
		logEntry := fmt.Sprintf("[%s] %s %s %s\n",
			change.Timestamp.Format("2006-01-02 15:04:05"),
			change.ResourceType,
			change.ResourceName,
			change.ChangeType)

		if _, err := cv.logFile.WriteString(logEntry); err != nil {
			fmt.Printf("Error writing to log file: %v\n", err)
		}
		// Flush immediately
		cv.logFile.Sync()
	}

	// Trigger title flash
	cv.flashTitle()
}

// formatValue formats a value for display in the changelog
func formatValue(value interface{}) string {
	if value == nil {
		return "-"
	}

	// If it's a NodeData struct, just return "present" or a simplified representation
	if _, ok := value.(NodeData); ok {
		return "present"
	}

	// For other types, return a simple string representation
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
