package cmd

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
)

// NodeDetailsView represents the node details view
type NodeDetailsView struct {
	table *tview.Table
	box   *tview.Box
	flex  *tview.Flex
}

// NewNodeDetailsView creates a new NodeDetailsView instance
func NewNodeDetailsView() *NodeDetailsView {
	detailsTable := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false)

	detailsBox := tview.NewBox().
		SetBorder(true).
		SetBorderColor(tcell.ColorGray).
		SetTitle("Node Details (Use mouse wheel or arrow keys to scroll)").
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

	return &NodeDetailsView{
		table: detailsTable,
		box:   detailsBox,
		flex:  detailsFlex,
	}
}

// GetTable returns the underlying table
func (dv *NodeDetailsView) GetTable() *tview.Table {
	return dv.table
}

// GetBox returns the details box
func (dv *NodeDetailsView) GetBox() *tview.Box {
	return dv.box
}

// GetFlex returns the flex container
func (dv *NodeDetailsView) GetFlex() *tview.Flex {
	return dv.flex
}

// ShowNodeDetails displays the details for a given node
func (dv *NodeDetailsView) ShowNodeDetails(node *corev1.Node) {
	// Clear and setup details table
	dv.table.Clear()

	// Basic Info
	row := 0
	dv.table.SetCell(row, 0, tview.NewTableCell("Basic Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Name").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Name).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Creation Time").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.CreationTimestamp.Format(time.RFC3339)).SetTextColor(tcell.ColorWhite))
	row++

	// System Info
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("System Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Machine ID").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.MachineID).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("System UUID").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.SystemUUID).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Boot ID").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.BootID).SetTextColor(tcell.ColorWhite))
	row++

	// Version Info
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Version Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Kernel Version").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.KernelVersion).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("OS Image").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.OSImage).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Container Runtime").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.ContainerRuntimeVersion).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Architecture").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.NodeInfo.Architecture).SetTextColor(tcell.ColorWhite))
	row++

	// Resource Info
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Resource Information").SetTextColor(tcell.ColorYellow).SetAttributes(tcell.AttrBold))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("CPU").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.Capacity.Cpu().String()).SetTextColor(tcell.ColorWhite))
	row++
	dv.table.SetCell(row, 0, tview.NewTableCell("Memory").SetTextColor(tcell.ColorSkyblue))
	dv.table.SetCell(row, 1, tview.NewTableCell(node.Status.Capacity.Memory().String()).SetTextColor(tcell.ColorWhite))
	row++

	// Labels and Annotations
	row++
	row = FormatMapAsRows(dv.table, row, "Labels", node.Labels)
	row++
	row = FormatMapAsRows(dv.table, row, "Annotations", node.Annotations)

	// Set initial selection for scrolling
	dv.table.Select(0, 0)

	// Update details box
	dv.box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		dv.table.SetRect(x+1, y+1, width-2, height-2)
		dv.table.Draw(screen)
		return x, y, width, height
	})
}
