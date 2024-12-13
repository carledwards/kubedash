package cmd

import (
	corev1 "k8s.io/api/core/v1"
)

// ClusterProvider handles cluster-level operations
type ClusterProvider interface {
	// GetClusterName returns the name of the cluster
	GetClusterName() string
}

// NodeProvider handles node-specific operations
type NodeProvider interface {
	// GetNodeMap returns the current node map
	GetNodeMap() map[string]*corev1.Node
}

// PodProvider handles pod-specific operations
type PodProvider interface {
	// GetPodsByNode returns pod data organized by node
	GetPodsByNode(includeNamespaces, excludeNamespaces map[string]bool) (map[string]map[string]PodInfo, error)
}

// K8sProvider combines all provider interfaces
type K8sProvider interface {
	ClusterProvider
	NodeProvider
	PodProvider

	// UpdateNodeData fetches the latest node and pod data
	// Returns:
	// - map[string]NodeData: node data indexed by node name
	// - map[string]map[string][]string: pod indicators by node and namespace
	// - error: any error that occurred
	UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error)
}
