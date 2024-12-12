package cmd

import (
	corev1 "k8s.io/api/core/v1"
)

// K8sDataProvider defines the interface for accessing Kubernetes data
type K8sDataProvider interface {
	// GetClusterName returns the name of the cluster
	GetClusterName() string

	// UpdateNodeData fetches the latest node and pod data
	// Returns:
	// - map[string]NodeData: node data indexed by node name
	// - map[string]map[string][]string: pod indicators by node and namespace
	// - error: any error that occurred
	UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error)

	// GetNodeMap returns the current node map
	GetNodeMap() map[string]*corev1.Node
}

// RealK8sDataProvider implements K8sDataProvider using actual Kubernetes cluster
type RealK8sDataProvider struct {
	client      *KubeClientWrapper
	clusterName string
	nodeMap     map[string]*corev1.Node
}

// NewRealK8sDataProvider creates a new RealK8sDataProvider
func NewRealK8sDataProvider() (*RealK8sDataProvider, error) {
	client, clusterName, err := NewKubeClient()
	if err != nil {
		return nil, err
	}

	return &RealK8sDataProvider{
		client:      client,
		clusterName: clusterName,
		nodeMap:     make(map[string]*corev1.Node),
	}, nil
}

func (p *RealK8sDataProvider) GetClusterName() string {
	return p.clusterName
}

func (p *RealK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	return UpdateNodeData(p.client.Clientset, p.nodeMap, includeNamespaces, excludeNamespaces)
}

func (p *RealK8sDataProvider) GetNodeMap() map[string]*corev1.Node {
	return p.nodeMap
}
