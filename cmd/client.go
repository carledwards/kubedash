package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// ArrayFlags represents a string array that can be used with flag package
type ArrayFlags []string

func (i *ArrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *ArrayFlags) Set(value string) error {
	// Handle comma-separated values
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			*i = append(*i, trimmed)
		}
	}
	return nil
}

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

// SortPodIndicators sorts pod indicators by color (RED, YELLOW, GREEN)
func SortPodIndicators(indicators []string) []string {
	// Define color priority (red = 0, yellow = 1, green = 2)
	colorPriority := map[string]int{
		"red":    0,
		"yellow": 1,
		"green":  2,
	}

	// Sort indicators by color
	sort.Slice(indicators, func(i, j int) bool {
		var color1, color2 string
		if strings.Contains(indicators[i], "[red]") {
			color1 = "red"
		} else if strings.Contains(indicators[i], "[yellow]") {
			color1 = "yellow"
		} else {
			color1 = "green"
		}

		if strings.Contains(indicators[j], "[red]") {
			color2 = "red"
		} else if strings.Contains(indicators[j], "[yellow]") {
			color2 = "yellow"
		} else {
			color2 = "green"
		}

		return colorPriority[color1] < colorPriority[color2]
	})

	return indicators
}

// GetNodeStatus returns the status of a node based on its conditions
func GetNodeStatus(node *corev1.Node) string {
	status := "Unknown"
	for _, cond := range node.Status.Conditions {
		if cond.Status == "True" && cond.Type != "Ready" {
			status = string(cond.Type)
			break
		}
	}
	// If no other condition is True, use Ready condition
	if status == "Unknown" {
		for _, cond := range node.Status.Conditions {
			if cond.Type == "Ready" {
				status = string(cond.Type)
				break
			}
		}
	}
	return status
}

// GetPodIndicator returns a pod's status indicator with color
func GetPodIndicator(pod *corev1.Pod) string {
	var restarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		restarts += containerStatus.RestartCount
	}

	if pod.Status.Phase == "Failed" {
		return "[red]■[white] "
	} else if restarts > 0 {
		return "[yellow]■[white] "
	}
	return "[green]■[white] "
}

// UpdateNodeData updates node and pod data
func UpdateNodeData(clientset *kubernetes.Clientset, nodeMap map[string]*corev1.Node, includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	// Get nodes
	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list nodes: %v", err)
	}

	// Build new state
	newState := make(map[string]NodeData)
	newNodeMap := make(map[string]*corev1.Node)

	// Track pod indicators by namespace for each node
	podsByNode := make(map[string]map[string][]string) // node -> namespace -> pod indicators

	// Process nodes
	for _, node := range nodes.Items {
		nodeCopy := node
		newNodeMap[node.Name] = &nodeCopy
		podsByNode[node.Name] = make(map[string][]string)

		// Get node status
		status := GetNodeStatus(&node)

		// Get pods for this node
		pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + node.Name,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list pods for node %s: %v", node.Name, err)
		}

		// Count total and filtered pods
		totalPods := len(pods.Items)
		filteredPods := 0

		// Create a map to store pod indicators by namespace
		podNamesByNamespace := make(map[string][]string)

		for _, pod := range pods.Items {
			// Skip if namespace is excluded
			if excludeNamespaces[pod.Namespace] {
				continue
			}
			// Skip if we have includes and this namespace isn't included
			if len(includeNamespaces) > 0 && !includeNamespaces[pod.Namespace] {
				continue
			}

			filteredPods++

			// Get pod indicator
			indicator := GetPodIndicator(&pod)

			// Initialize slice if needed
			if podNamesByNamespace[pod.Namespace] == nil {
				podNamesByNamespace[pod.Namespace] = make([]string, 0)
			}
			// Add pod indicator to the slice
			podNamesByNamespace[pod.Namespace] = append(podNamesByNamespace[pod.Namespace], indicator)
		}

		// Sort and store pod indicators for each namespace
		for ns, indicators := range podNamesByNamespace {
			// Sort indicators by color (RED, YELLOW, GREEN)
			sortedIndicators := SortPodIndicators(indicators)
			podsByNode[node.Name][ns] = sortedIndicators
		}

		// Format pod count string
		podCountStr := fmt.Sprintf("%d", totalPods)
		if len(includeNamespaces) > 0 || len(excludeNamespaces) > 0 {
			podCountStr = fmt.Sprintf("%d (%d)", filteredPods, totalPods)
		}

		// Calculate node age
		age := FormatDuration(time.Since(node.CreationTimestamp.Time))

		// Store new state
		newState[node.Name] = NodeData{
			Name:     node.Name,
			Status:   status,
			Version:  node.Status.NodeInfo.KubeletVersion,
			PodCount: podCountStr,
			Age:      age,
		}
	}

	// Update node map
	for k := range nodeMap {
		delete(nodeMap, k)
	}
	for k, v := range newNodeMap {
		nodeMap[k] = v
	}

	return newState, podsByNode, nil
}
