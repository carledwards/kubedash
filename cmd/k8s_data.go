package cmd

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// KubeClientWrapper wraps kubernetes clientset and configuration
type KubeClientWrapper struct {
	Clientset *kubernetes.Clientset
	Config    *api.Config
}

// NewKubeClient creates a new KubeClient with the current context
func NewKubeClient() (*KubeClientWrapper, string, error) {
	// Get current context name
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get client config: %v", err)
	}

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get raw config: %v", err)
	}

	currentContext := rawConfig.CurrentContext
	contextInfo := rawConfig.Contexts[currentContext]
	clusterName := contextInfo.Cluster
	if clusterName == "" {
		clusterName = currentContext
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create clientset: %v", err)
	}

	return &KubeClientWrapper{
		Clientset: clientset,
		Config:    &rawConfig,
	}, clusterName, nil
}

// RealK8sDataProvider implements K8sProvider using actual Kubernetes cluster
type RealK8sDataProvider struct {
	BaseK8sDataProvider
	client      *KubeClientWrapper
	clusterName string
	rawData     map[string]RawNodeData
	podsByNode  map[string]map[string][]string
}

// NewRealK8sDataProvider creates a new RealK8sDataProvider
func NewRealK8sDataProvider() (*RealK8sDataProvider, error) {
	client, clusterName, err := NewKubeClient()
	if err != nil {
		return nil, err
	}

	return &RealK8sDataProvider{
		BaseK8sDataProvider: BaseK8sDataProvider{
			nodeMap: make(map[string]*corev1.Node),
		},
		client:      client,
		clusterName: clusterName,
		rawData:     make(map[string]RawNodeData),
		podsByNode:  make(map[string]map[string][]string),
	}, nil
}

// GetClusterName implements ClusterProvider interface
func (p *RealK8sDataProvider) GetClusterName() string {
	return p.clusterName
}

// GetPodsByNode returns the current pod data by node
func (p *RealK8sDataProvider) GetPodsByNode() map[string]map[string][]string {
	return p.podsByNode
}

// GetRawData implements K8sProvider interface
func (p *RealK8sDataProvider) GetRawData() (map[string]RawNodeData, error) {
	return p.rawData, nil
}

// GetFilteredData implements K8sProvider interface
func (p *RealK8sDataProvider) GetFilteredData(criteria FilterCriteria) (map[string]NodeData, map[string]map[string][]string, error) {
	return p.filterAndTransformData(p.rawData, criteria)
}

// UpdateNodeData implements K8sProvider interface
func (p *RealK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
	defer cancel()

	// Get nodes
	nodes, err := p.client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list nodes (timeout %v): %v", APITimeout, err)
	}

	// Get pods from all namespaces
	pods, err := p.client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list pods (timeout %v): %v", APITimeout, err)
	}

	// Build raw data
	p.rawData = make(map[string]RawNodeData)
	for i := range nodes.Items {
		node := &nodes.Items[i]
		p.nodeMap[node.Name] = node
		p.rawData[node.Name] = RawNodeData{
			Node: node,
			Pods: make(map[string]*corev1.Pod),
		}
	}

	// Add pods to raw data
	for i := range pods.Items {
		pod := &pods.Items[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}
		if data, exists := p.rawData[nodeName]; exists {
			data.Pods[pod.Name] = pod
			p.rawData[nodeName] = data
		}
	}

	// Apply initial filtering
	criteria := FilterCriteria{
		IncludeNamespaces: includeNamespaces,
		ExcludeNamespaces: excludeNamespaces,
		SearchQuery:       "",
	}

	return p.filterAndTransformData(p.rawData, criteria)
}
