package cmd

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Config holds the application configuration
type Config struct {
	IncludeNamespaces map[string]bool
	ExcludeNamespaces map[string]bool
	UseMockData       bool
	LogFilePath       string
}

// App represents the main application
type App struct {
	config         *Config
	provider       K8sProvider
	ui             *UI
	stateCache     *StateCache
	isRefreshing   atomic.Bool
	spinnerIndex   atomic.Int32
	showingDetails bool
	hasError       atomic.Bool
}

// NewApp creates a new application instance
func NewApp(config *Config) (*App, error) {
	var provider K8sProvider
	var err error

	if config.UseMockData {
		provider = NewMockK8sDataProvider()
	} else {
		provider, err = NewRealK8sDataProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to create K8s provider: %v", err)
		}
	}

	app := &App{
		config:     config,
		provider:   provider,
		stateCache: NewStateCache(),
	}

	// Create UI components
	app.ui = NewUI(app)
	if err := app.ui.Setup(); err != nil {
		return nil, fmt.Errorf("failed to setup UI: %v", err)
	}

	return app, nil
}

// Run starts the application
func (a *App) Run() error {
	fmt.Println("Starting application...")

	// Initial data load without changelog updates
	nodeData, podsByNode, err := a.provider.UpdateNodeData(
		a.config.IncludeNamespaces,
		a.config.ExcludeNamespaces,
	)
	if err != nil {
		return fmt.Errorf("failed to load initial data: %v", err)
	}

	// Update nodeView's map with the provider's map
	for k, v := range a.provider.GetNodeMap() {
		a.ui.nodeView.GetNodeMap()[k] = v
	}

	// Update UI with initial data
	a.ui.UpdateTable(nodeData, podsByNode)

	// Initialize state cache after UI is ready
	for nodeName, data := range nodeData {
		a.stateCache.Put(nodeName, ResourceState{
			Data:      data,
			Timestamp: time.Now(),
		})
	}

	// Set up auto-refresh ticker
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if err := a.refreshData(); err != nil {
				if !a.hasError.Load() {
					a.hasError.Store(true)
					a.ui.app.QueueUpdateDraw(func() {
						a.ui.ShowErrorMessage()
					})
				}
				// Start background retry if not already running
				go a.retryInBackground()
			}
		}
	}()
	defer ticker.Stop()

	// Spinner animation ticker
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	go func() {
		for range spinnerTicker.C {
			if a.isRefreshing.Load() {
				a.spinnerIndex.Add(1)
				a.ui.app.QueueUpdateDraw(func() {})
			}
		}
	}()
	defer spinnerTicker.Stop()

	fmt.Println("Starting UI...")

	// Run the application
	if err := a.ui.app.Run(); err != nil {
		return fmt.Errorf("application error: %v", err)
	}

	return nil
}

// retryInBackground attempts to refresh data in the background
func (a *App) retryInBackground() {
	if !a.hasError.Load() {
		return // Don't retry if there's no error
	}

	retryTicker := time.NewTicker(5 * time.Second)
	defer retryTicker.Stop()

	for range retryTicker.C {
		if !a.hasError.Load() {
			return // Stop retrying if error is cleared
		}

		if err := a.refreshData(); err == nil {
			// Success - dismiss error message and return
			a.hasError.Store(false)
			a.ui.app.QueueUpdateDraw(func() {
				a.ui.DismissErrorMessage()
			})
			return
		}
	}
}

// refreshData updates the node and pod data
func (a *App) refreshData() error {
	if !a.showingDetails && !a.isRefreshing.Load() {
		a.isRefreshing.Store(true)
		defer a.isRefreshing.Store(false)

		nodeData, podsByNode, err := a.provider.UpdateNodeData(
			a.config.IncludeNamespaces,
			a.config.ExcludeNamespaces,
		)
		if err != nil {
			return fmt.Errorf("failed to refresh data: %v", err)
		}

		// Check for changes and update changelog
		for nodeName, newData := range nodeData {
			changes := a.stateCache.Compare(nodeName, ResourceState{
				Data:      newData,
				Timestamp: time.Now(),
			})
			for _, change := range changes {
				a.ui.changeLogView.AddChange(change)
			}
		}

		// Check for removed nodes
		for nodeName := range a.ui.nodeView.GetNodeMap() {
			if _, exists := nodeData[nodeName]; !exists {
				changes := a.stateCache.Compare(nodeName, ResourceState{
					Data:      nil,
					Timestamp: time.Now(),
				})
				for _, change := range changes {
					a.ui.changeLogView.AddChange(change)
				}
			}
		}

		// Update nodeView's map
		for k := range a.ui.nodeView.GetNodeMap() {
			delete(a.ui.nodeView.GetNodeMap(), k)
		}
		for k, v := range a.provider.GetNodeMap() {
			a.ui.nodeView.GetNodeMap()[k] = v
		}

		a.ui.UpdateTable(nodeData, podsByNode)
	}

	return nil
}

// GetProvider returns the K8s provider
func (a *App) GetProvider() K8sProvider {
	return a.provider
}

// GetSpinnerChar returns the current spinner character
func (a *App) GetSpinnerChar() rune {
	spinnerChars := []rune{'-', '\\', '|', '/'}
	return spinnerChars[int(a.spinnerIndex.Load())%len(spinnerChars)]
}

// IsRefreshing returns whether the app is currently refreshing data
func (a *App) IsRefreshing() bool {
	return a.isRefreshing.Load()
}

// SetShowingDetails sets whether the details view is being shown
func (a *App) SetShowingDetails(showing bool) {
	a.showingDetails = showing
}

// IsShowingDetails returns whether the details view is being shown
func (a *App) IsShowingDetails() bool {
	return a.showingDetails
}