package cmd

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PodDetailsView represents the pod details view
type PodDetailsView struct {
	table *tview.Table
	box   *tview.Box
	flex  *tview.Flex
}

// NewPodDetailsView creates a new PodDetailsView instance
func NewPodDetailsView() *PodDetailsView {
	detailsTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	detailsBox := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle("Pod Details (Use mouse wheel or arrow keys to scroll)").
		SetBorderAttributes(tcell.AttrDim)

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

	return &PodDetailsView{
		table: detailsTable,
		box:   detailsBox,
		flex:  detailsFlex,
	}
}

// GetTable returns the underlying table
func (dv *PodDetailsView) GetTable() *tview.Table {
	return dv.table
}

// GetBox returns the details box
func (dv *PodDetailsView) GetBox() *tview.Box {
	return dv.box
}

// GetFlex returns the flex container
func (dv *PodDetailsView) GetFlex() *tview.Flex {
	return dv.flex
}

// ShowPodDetails displays the details for pods on a given node and namespace
func (dv *PodDetailsView) ShowPodDetails(nodeName string, namespace string, pods map[string]PodInfo) {
	// Clear and setup details table
	dv.table.Clear()

	// Set up header row
	headers := []string{"Pod Name", "Status", "Containers Ready", "Restarts", "Container Status"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorWhite).
			SetSelectable(false).
			SetExpansion(1).
			SetAttributes(tcell.AttrBold)
		dv.table.SetCell(0, i, cell)
	}

	// Update title with node and namespace info
	dv.box.SetTitle(fmt.Sprintf("Pod Details - Node: %s, Namespace: %s (Use mouse wheel or arrow keys to scroll)", nodeName, namespace))

	// Add pod rows
	row := 1
	for podName, podInfo := range pods {
		// Pod Name
		dv.table.SetCell(row, 0, tview.NewTableCell(podName).
			SetTextColor(tcell.ColorSkyblue))

		// Status
		statusColor := tcell.ColorGreen
		if podInfo.Status != PodStatusRunning {
			statusColor = tcell.ColorRed
		}
		dv.table.SetCell(row, 1, tview.NewTableCell(podInfo.Status).
			SetTextColor(statusColor))

		// Containers Ready
		readyCount := 0
		totalCount := len(podInfo.ContainerInfo)
		for _, container := range podInfo.ContainerInfo {
			if container.Status == PodStatusRunning {
				readyCount++
			}
		}
		dv.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d/%d", readyCount, totalCount)).
			SetTextColor(tcell.ColorWhite))

		// Restarts
		restartColor := tcell.ColorGreen
		if podInfo.RestartCount > 0 {
			restartColor = tcell.ColorYellow
		}
		if podInfo.RestartCount > 5 {
			restartColor = tcell.ColorRed
		}
		dv.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d", podInfo.RestartCount)).
			SetTextColor(restartColor))

		// Container Status
		var containerStatus string
		for containerName, container := range podInfo.ContainerInfo {
			containerStatus += fmt.Sprintf("%s: %s\n", containerName, container.Status)
		}
		dv.table.SetCell(row, 4, tview.NewTableCell(containerStatus).
			SetTextColor(tcell.ColorWhite))

		row++
	}

	// Set initial selection for scrolling
	dv.table.Select(1, 0)

	// Update details box
	dv.box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		dv.table.SetRect(x+1, y+1, width-2, height-2)
		dv.table.Draw(screen)
		return x, y, width, height
	})
}
