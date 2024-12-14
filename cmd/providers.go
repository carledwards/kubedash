package cmd

import (
	corev1 "k8s.io/api/core/v1"
)

// ClusterProvider defines methods for getting cluster information
type ClusterProvider interface {
	// GetClusterName returns the name of the current cluster
	GetClusterName() string

	// GetNodeMap returns the current node map
	GetNodeMap() map[string]*corev1.Node
}

// K8sProvider combines all provider interfaces
type K8sProvider interface {
	ClusterProvider

	// UpdateNodeData fetches the latest node and pod data
	// Parameters:
	// - includeNamespaces: map of namespaces to include (empty means include all)
	// - excludeNamespaces: map of namespaces to exclude
	// Returns:
	// - map[string]NodeData: filtered node data
	// - map[string]map[string][]string: filtered pod indicators by node and namespace
	// - error: any error that occurred
	UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error)

	// GetRawData returns the unfiltered node and pod data
	// Returns:
	// - map[string]RawNodeData: raw node and pod data
	// - error: any error that occurred
	GetRawData() (map[string]RawNodeData, error)

	// GetFilteredData returns filtered node and pod data based on criteria
	// Parameters:
	// - criteria: filtering criteria to apply
	// Returns:
	// - map[string]NodeData: filtered node data
	// - map[string]map[string][]string: filtered pod indicators by node and namespace
	// - error: any error that occurred
	GetFilteredData(criteria FilterCriteria) (map[string]NodeData, map[string]map[string][]string, error)

	// GetPodsByNode returns the current pod data by node
	GetPodsByNode() map[string]map[string][]string
}
