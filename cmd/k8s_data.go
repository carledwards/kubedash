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
	}, nil
}

// GetClusterName implements ClusterProvider interface
func (p *RealK8sDataProvider) GetClusterName() string {
	return p.clusterName
}

// GetPodsByNode implements PodProvider interface
func (p *RealK8sDataProvider) GetPodsByNode(includeNamespaces, excludeNamespaces map[string]bool) (map[string]map[string]PodInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), APITimeout)
	defer cancel()

	pods, err := p.client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods (timeout %v): %v", APITimeout, err)
	}

	podsByNode := make(map[string]map[string]PodInfo)
	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}

		ns := pod.Namespace
		if excludeNamespaces[ns] {
			continue
		}
		if len(includeNamespaces) > 0 && !includeNamespaces[ns] {
			continue
		}

		if _, exists := podsByNode[nodeName]; !exists {
			podsByNode[nodeName] = make(map[string]PodInfo)
		}

		podsByNode[nodeName][pod.Name] = GetPodInfo(&pod)
	}

	return podsByNode, nil
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

	return p.ProcessNodeData(nodes.Items, pods.Items, includeNamespaces, excludeNamespaces)
}
