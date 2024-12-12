package cmd

import (
	"fmt"
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
	ResourceName string // Name of the resource that changed
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
			ResourceType: "Node",
			ResourceName: key,
			ChangeType:   "Added",
			NewValue:     newState.Data,
			Timestamp:    time.Now(),
		})
	} else if newState.Data == nil {
		// Resource removed
		changes = append(changes, ChangeEvent{
			ResourceType: "Node",
			ResourceName: key,
			ChangeType:   "Removed",
			OldValue:     oldState.Data,
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

		// Check node status
		if oldData.Status != newData.Status {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ResourceName: key,
				ChangeType:   "Modified",
				Field:        "Status",
				OldValue:     oldData.Status,
				NewValue:     newData.Status,
				Timestamp:    time.Now(),
			})
		}

		// Check node version
		if oldData.Version != newData.Version {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ResourceName: key,
				ChangeType:   "Modified",
				Field:        "Version",
				OldValue:     oldData.Version,
				NewValue:     newData.Version,
				Timestamp:    time.Now(),
			})
		}

		// Check pod count
		if oldData.PodCount != newData.PodCount {
			changes = append(changes, ChangeEvent{
				ResourceType: "Node",
				ResourceName: key,
				ChangeType:   "Modified",
				Field:        "PodCount",
				OldValue:     oldData.PodCount,
				NewValue:     newData.PodCount,
				Timestamp:    time.Now(),
			})
		}

		// Compare pod states
		for podName, newPod := range newData.Pods {
			oldPod, exists := oldData.Pods[podName]
			if !exists {
				// New pod added
				changes = append(changes, ChangeEvent{
					ResourceType: "Pod",
					ResourceName: fmt.Sprintf("%s/%s", key, podName),
					ChangeType:   "Added",
					Field:        "Status",
					NewValue:     newPod.Status,
					Timestamp:    time.Now(),
				})
			} else {
				// Check pod status changes
				if oldPod.Status != newPod.Status {
					changes = append(changes, ChangeEvent{
						ResourceType: "Pod",
						ResourceName: fmt.Sprintf("%s/%s", key, podName),
						ChangeType:   "Modified",
						Field:        "Status",
						OldValue:     oldPod.Status,
						NewValue:     newPod.Status,
						Timestamp:    time.Now(),
					})
				}

				// Check pod restart count changes
				if oldPod.RestartCount != newPod.RestartCount {
					changes = append(changes, ChangeEvent{
						ResourceType: "Pod",
						ResourceName: fmt.Sprintf("%s/%s", key, podName),
						ChangeType:   "Modified",
						Field:        "RestartCount",
						OldValue:     oldPod.RestartCount,
						NewValue:     newPod.RestartCount,
						Timestamp:    time.Now(),
					})
				}

				// Check container changes
				for containerName, newContainer := range newPod.ContainerInfo {
					oldContainer, exists := oldPod.ContainerInfo[containerName]
					if !exists {
						// New container added
						changes = append(changes, ChangeEvent{
							ResourceType: "Container",
							ResourceName: fmt.Sprintf("%s/%s/%s", key, podName, containerName),
							ChangeType:   "Added",
							Field:        "Status",
							NewValue:     newContainer.Status,
							Timestamp:    time.Now(),
						})
					} else {
						// Check container status changes
						if oldContainer.Status != newContainer.Status {
							changes = append(changes, ChangeEvent{
								ResourceType: "Container",
								ResourceName: fmt.Sprintf("%s/%s/%s", key, podName, containerName),
								ChangeType:   "Modified",
								Field:        "Status",
								OldValue:     oldContainer.Status,
								NewValue:     newContainer.Status,
								Timestamp:    time.Now(),
							})
						}

						// Check container restart count changes
						if oldContainer.RestartCount != newContainer.RestartCount {
							changes = append(changes, ChangeEvent{
								ResourceType: "Container",
								ResourceName: fmt.Sprintf("%s/%s/%s", key, podName, containerName),
								ChangeType:   "Modified",
								Field:        "RestartCount",
								OldValue:     oldContainer.RestartCount,
								NewValue:     newContainer.RestartCount,
								Timestamp:    time.Now(),
							})
						}
					}
				}

				// Check for removed containers
				for containerName := range oldPod.ContainerInfo {
					if _, exists := newPod.ContainerInfo[containerName]; !exists {
						changes = append(changes, ChangeEvent{
							ResourceType: "Container",
							ResourceName: fmt.Sprintf("%s/%s/%s", key, podName, containerName),
							ChangeType:   "Removed",
							Field:        "Status",
							OldValue:     oldPod.ContainerInfo[containerName].Status,
							Timestamp:    time.Now(),
						})
					}
				}
			}
		}

		// Check for removed pods
		for podName, oldPod := range oldData.Pods {
			if _, exists := newData.Pods[podName]; !exists {
				changes = append(changes, ChangeEvent{
					ResourceType: "Pod",
					ResourceName: fmt.Sprintf("%s/%s", key, podName),
					ChangeType:   "Removed",
					Field:        "Status",
					OldValue:     oldPod.Status,
					Timestamp:    time.Now(),
				})
			}
		}
	}

	// Update cache with new state if it's not a removal
	if newState.Data != nil {
		sc.cache[key] = newState
	} else {
		delete(sc.cache, key)
	}

	return changes
}
