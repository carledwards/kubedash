package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (p *RealK8sDataProvider) GetNodeMap() map[string]*corev1.Node {
	return p.nodeMap
}

// getPodInfo extracts PodInfo from a Kubernetes Pod
func getPodInfo(pod *corev1.Pod) PodInfo {
	podInfo := PodInfo{
		Name:          pod.Name,
		Status:        string(pod.Status.Phase),
		RestartCount:  0,
		ContainerInfo: make(map[string]ContainerInfo),
	}

	// Get container information
	for _, container := range pod.Spec.Containers {
		var containerStatus *corev1.ContainerStatus
		for i := range pod.Status.ContainerStatuses {
			if pod.Status.ContainerStatuses[i].Name == container.Name {
				containerStatus = &pod.Status.ContainerStatuses[i]
				break
			}
		}

		status := "Unknown"
		restartCount := 0
		if containerStatus != nil {
			if containerStatus.State.Running != nil {
				status = "Running"
			} else if containerStatus.State.Waiting != nil {
				status = containerStatus.State.Waiting.Reason
			} else if containerStatus.State.Terminated != nil {
				status = containerStatus.State.Terminated.Reason
			}
			restartCount = int(containerStatus.RestartCount)
			podInfo.RestartCount += restartCount
		}

		podInfo.ContainerInfo[container.Name] = ContainerInfo{
			Status:       status,
			RestartCount: restartCount,
		}
	}

	// Handle terminating state
	if pod.DeletionTimestamp != nil {
		podInfo.Status = "Terminating"
	}

	return podInfo
}

// UpdateNodeData implements K8sDataProvider interface
func (p *RealK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	ctx := context.Background()
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)

	// Get nodes
	nodes, err := p.client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	// Clear and update node map
	for k := range p.nodeMap {
		delete(p.nodeMap, k)
	}
	for i := range nodes.Items {
		node := &nodes.Items[i]
		p.nodeMap[node.Name] = node
	}

	// Get pods from all namespaces
	pods, err := p.client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list pods: %v", err)
	}

	// Initialize node data structures
	for _, node := range nodes.Items {
		nodeStatus := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					nodeStatus = "Ready"
				}
				break
			}
		}

		data := NodeData{
			Name:          node.Name,
			Status:        nodeStatus,
			Version:       node.Status.NodeInfo.KubeletVersion,
			Age:           FormatDuration(time.Since(node.CreationTimestamp.Time)),
			PodCount:      "0",
			PodIndicators: "",
			Pods:          make(map[string]PodInfo),
		}
		nodeData[node.Name] = data
		podsByNode[node.Name] = make(map[string][]string)
	}

	// Process pods
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

		// Initialize namespace map if needed
		if _, exists := podsByNode[nodeName][ns]; !exists {
			podsByNode[nodeName][ns] = make([]string, 0)
		}

		// Get pod status indicator
		indicator := "[green]■[white] " // default to success
		switch {
		case pod.Status.Phase == corev1.PodFailed:
			indicator = "[red]■[white] "
		case pod.Status.Phase == corev1.PodPending || pod.DeletionTimestamp != nil:
			indicator = "[yellow]■[white] "
		case pod.Status.Phase == corev1.PodRunning:
			allReady := true
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
					allReady = false
					break
				}
			}
			if !allReady {
				indicator = "[yellow]■[white] "
			}
		}

		podsByNode[nodeName][ns] = append(podsByNode[nodeName][ns], indicator)

		// Update pod info in node data
		if data, exists := nodeData[nodeName]; exists {
			podInfo := getPodInfo(&pod)
			data.Pods[pod.Name] = podInfo
			data.PodCount = fmt.Sprintf("%d", len(data.Pods))
			data.PodIndicators = strings.Join(podsByNode[nodeName][ns], "")
			nodeData[nodeName] = data
		}
	}

	// Sort pod indicators for consistent display
	for nodeName := range podsByNode {
		for ns := range podsByNode[nodeName] {
			podsByNode[nodeName][ns] = SortPodIndicators(podsByNode[nodeName][ns])
		}
	}

	return nodeData, podsByNode, nil
}
