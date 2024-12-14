package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	corev1 "k8s.io/api/core/v1"
)

// LogView represents a full-screen log streaming view
type LogView struct {
	textView    *tview.TextView
	flex        *tview.Flex
	pod         *PodInfo
	stopChan    chan struct{}
	app         *tview.Application
	previousApp tview.Primitive
	autoScroll  bool
}

// NewLogView creates a new LogView instance
func NewLogView() *LogView {
	logView := &LogView{
		textView: tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true).
			SetWrap(true),
		stopChan:   make(chan struct{}),
		autoScroll: true,
	}

	// Create a flex container for the log view
	logView.flex = tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(logView.textView, 0, 1, true)

	// Add border with title and instructions
	logView.textView.SetBorder(true)
	logView.textView.SetTitle(" Pod Logs (Press Esc to exit, ↑/↓ to scroll, Space to toggle auto-scroll) ")

	// Set up input handling for the text view
	logView.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			logView.Stop()
			if logView.app != nil && logView.previousApp != nil {
				logView.app.SetRoot(logView.previousApp, true)
			}
			return nil
		case tcell.KeyUp:
			logView.autoScroll = false
			row, _ := logView.textView.GetScrollOffset()
			if row > 0 {
				logView.textView.ScrollTo(row-1, 0)
			}
			return nil
		case tcell.KeyDown:
			row, _ := logView.textView.GetScrollOffset()
			logView.textView.ScrollTo(row+1, 0)
			return nil
		case tcell.KeyPgUp:
			logView.autoScroll = false
			row, _ := logView.textView.GetScrollOffset()
			logView.textView.ScrollTo(row-10, 0)
			return nil
		case tcell.KeyPgDn:
			row, _ := logView.textView.GetScrollOffset()
			logView.textView.ScrollTo(row+10, 0)
			return nil
		case tcell.KeyRune:
			if event.Rune() == ' ' {
				logView.autoScroll = !logView.autoScroll
				if logView.autoScroll {
					logView.textView.ScrollToEnd()
				}
				return nil
			}
		}
		return event
	})

	return logView
}

// SetApplication sets the tview application reference
func (l *LogView) SetApplication(app *tview.Application) {
	l.app = app
}

// SetPreviousApp sets the previous app to return to when closing logs
func (l *LogView) SetPreviousApp(app tview.Primitive) {
	l.previousApp = app
}

// GetFlex returns the flex container
func (l *LogView) GetFlex() *tview.Flex {
	return l.flex
}

// ShowPodLogs displays logs for the specified pod
func (l *LogView) ShowPodLogs(k8s *KubeClientWrapper, podInfo *PodInfo) {
	l.pod = podInfo
	l.textView.Clear()
	l.textView.SetTitle(fmt.Sprintf(" Pod Logs: %s/%s (Press Esc to exit, ↑/↓ to scroll, Space to toggle auto-scroll) ", podInfo.Namespace, podInfo.Name))

	// Stop any existing log stream
	if l.stopChan != nil {
		close(l.stopChan)
	}
	l.stopChan = make(chan struct{})
	l.autoScroll = true

	// Start streaming logs
	go l.streamLogs(k8s, podInfo)
}

// streamLogs continuously streams logs from the pod
func (l *LogView) streamLogs(k8s *KubeClientWrapper, podInfo *PodInfo) {
	podLogOpts := &corev1.PodLogOptions{
		Follow:    true,
		TailLines: new(int64), // Start from the end of logs
	}
	*podLogOpts.TailLines = 1000 // Show last 1000 lines initially

	req := k8s.Clientset.CoreV1().Pods(podInfo.Namespace).GetLogs(podInfo.Name, podLogOpts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		l.textView.SetText(fmt.Sprintf("[red]Error getting pod logs: %v", err))
		return
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-l.stopChan:
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					l.textView.Write([]byte(fmt.Sprintf("[red]Error reading logs: %v\n", err)))
				}
				return
			}

			l.textView.Write([]byte(line))

			// Auto-scroll to bottom if enabled
			if l.autoScroll && l.app != nil {
				l.app.QueueUpdateDraw(func() {
					l.textView.ScrollToEnd()
				})
			}
		}
	}
}

// Stop stops the log streaming
func (l *LogView) Stop() {
	if l.stopChan != nil {
		close(l.stopChan)
	}
}
