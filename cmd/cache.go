package cmd

import (
	"sync"
	"time"
)

// ResourceState represents the state of a resource at a point in time
type ResourceState struct {
	Generation int64
	Data       interface{}
	Metadata   map[string]string
	Timestamp  time.Time
}

// ChangeEvent represents a detected change in a resource
type ChangeEvent struct {
	ResourceType string // "Node", "Pod", etc
	ChangeType   string // "Added", "Removed", "Modified"
	Field        string // Specific field that changed
	OldValue     interface{}
	NewValue     interface{}
	Timestamp    time.Time
}

// StateCache provides thread-safe caching and comparison of resource states
type StateCache struct {
	mu    sync.RWMutex
	cache map[string]ResourceState
}

// NewStateCache creates a new StateCache instance
func NewStateCache() *StateCache {
	return &StateCache{
		cache: make(map[string]ResourceState),
	}
}

// Put stores a resource state in the cache
func (sc *StateCache) Put(key string, state ResourceState) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.cache[key] = state
}

// Get retrieves a resource state from the cache
func (sc *StateCache) Get(key string) (ResourceState, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	state, exists := sc.cache[key]
	return state, exists
}

// Compare compares a new state with the cached state and returns changes
func (sc *StateCache) Compare(key string, newState ResourceState) []ChangeEvent {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	oldState, exists := sc.cache[key]
	var changes []ChangeEvent

	if !exists {
		// New resource added
		changes = append(changes, ChangeEvent{
			ResourceType: "Node", // We'll make this dynamic later
			ChangeType:   "Added",
			NewValue:     newState.Data,
			Timestamp:    time.Now(),
		})
	} else {
		// Compare specific fields we care about
		oldData, ok := oldState.Data.(NodeData)
		if !ok {
			return changes
		}

		newData, ok := newState.Data.(NodeData)
		if !ok {
			return changes
		}

		// Check specific fields
		if oldData.Status != newData.Status {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ChangeType:   "Modified",
				Field:        "Status",
				OldValue:     oldData.Status,
				NewValue:     newData.Status,
				Timestamp:    time.Now(),
			})
		}

		if oldData.Version != newData.Version {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ChangeType:   "Modified",
				Field:        "Version",
				OldValue:     oldData.Version,
				NewValue:     newData.Version,
				Timestamp:    time.Now(),
			})
		}

		if oldData.PodCount != newData.PodCount {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ChangeType:   "Modified",
				Field:        "PodCount",
				OldValue:     oldData.PodCount,
				NewValue:     newData.PodCount,
				Timestamp:    time.Now(),
			})
		}

		if oldData.PodIndicators != newData.PodIndicators {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ChangeType:   "Modified",
				Field:        "PodIndicators",
				OldValue:     oldData.PodIndicators,
				NewValue:     newData.PodIndicators,
				Timestamp:    time.Now(),
			})
		}
	}

	// Update cache with new state
	sc.cache[key] = newState
	return changes
}
