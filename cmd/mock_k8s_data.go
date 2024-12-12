package cmd

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockK8sDataProvider implements K8sDataProvider using mock data
type MockK8sDataProvider struct {
	nodeMap     map[string]*corev1.Node
	clusterName string
	podStates   map[string]map[string]PodInfo
	rand        *rand.Rand // node -> pod name -> pod info
}

// NewMockK8sDataProvider creates a new MockK8sDataProvider
func NewMockK8sDataProvider() *MockK8sDataProvider {
	return &MockK8sDataProvider{
		nodeMap:     make(map[string]*corev1.Node),
		clusterName: "mock-cluster",
		podStates:   make(map[string]map[string]PodInfo),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *MockK8sDataProvider) GetClusterName() string {
	return p.clusterName
}

func (p *MockK8sDataProvider) GetNodeMap() map[string]*corev1.Node {
	return p.nodeMap
}

func createMockNodeConditions(status string) []corev1.NodeCondition {
	now := metav1.Now()
	conditions := []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             corev1.ConditionStatus(status),
			LastHeartbeatTime:  now,
			LastTransitionTime: now,
			Reason:             "KubeletReady",
			Message:            "kubelet is ready",
		},
		{
			Type:               corev1.NodeMemoryPressure,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  now,
			LastTransitionTime: now,
			Reason:             "KubeletHasSufficientMemory",
			Message:            "kubelet has sufficient memory available",
		},
		{
			Type:               corev1.NodeDiskPressure,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  now,
			LastTransitionTime: now,
			Reason:             "KubeletHasNoDiskPressure",
			Message:            "kubelet has no disk pressure",
		},
		{
			Type:               corev1.NodePIDPressure,
			Status:             corev1.ConditionFalse,
			LastHeartbeatTime:  now,
			LastTransitionTime: now,
			Reason:             "KubeletHasSufficientPID",
			Message:            "kubelet has sufficient PID available",
		},
	}
	return conditions
}

func createMockPodInfo(r *rand.Rand, podName string) PodInfo {
	// Possible pod statuses
	statuses := []string{"Running", "Pending", "Failed", "Terminating"}
	status := statuses[r.Intn(len(statuses))]

	// Create container info
	containers := make(map[string]ContainerInfo)
	containerCount := r.Intn(2) + 1 // 1-2 containers per pod

	for i := 0; i < containerCount; i++ {
		containerName := fmt.Sprintf("%s-container-%d", podName, i)
		containers[containerName] = ContainerInfo{
			Status:       status,
			RestartCount: r.Intn(5), // 0-4 restarts
		}
	}

	return PodInfo{
		Name:          podName,
		Status:        status,
		RestartCount:  r.Intn(10), // 0-9 restarts
		ContainerInfo: containers,
	}
}

func (p *MockK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	r := p.rand

	// Reuse existing states
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)
	namespaces := []string{"default", "kube-system", "monitoring"}

	// Randomly pick a node to change
	nodeNames := []string{"node1", "node2", "node3"}
	randomNode := nodeNames[r.Intn(len(nodeNames))]

	// Simulate a single change
	changeType := r.Intn(3) // 0 = pod addition, 1 = pod status change, 2 = node readiness change
	fmt.Print(changeType)

	switch changeType {
	case 0: // Add a new pod
		namespace := namespaces[r.Intn(len(namespaces))]
		if excludeNamespaces[namespace] || (len(includeNamespaces) > 0 && !includeNamespaces[namespace]) {
			break
		}

		podName := fmt.Sprintf("%s-pod-%s-%d", randomNode, namespace, len(p.podStates[randomNode])+1)
		podInfo := createMockPodInfo(r, podName)

		if _, exists := p.podStates[randomNode]; !exists {
			p.podStates[randomNode] = make(map[string]PodInfo)
		}
		p.podStates[randomNode][podName] = podInfo
		podsByNode[randomNode] = map[string][]string{
			namespace: {fmt.Sprintf("[green]■[white]")},
		}

	case 1: // Update a pod status
		if len(p.podStates[randomNode]) > 0 {
			podKeys := make([]string, 0, len(p.podStates[randomNode]))
			for podName := range p.podStates[randomNode] {
				podKeys = append(podKeys, podName)
			}
			randomPod := podKeys[r.Intn(len(podKeys))]
			updatedPod := createMockPodInfo(r, randomPod)
			p.podStates[randomNode][randomPod] = updatedPod
		}

	case 2: // Change node readiness
		node, exists := p.nodeMap[randomNode]
		if !exists {
			// Initialize a new node if it doesn't exist
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: randomNode,
				},
				Status: corev1.NodeStatus{
					Conditions: createMockNodeConditions("False"), // Initialize with default conditions
				},
			}
			p.nodeMap[randomNode] = node // Persist the new node
		}

		// Safely toggle the first condition
		if len(node.Status.Conditions) > 0 {
			if node.Status.Conditions[0].Status == corev1.ConditionTrue {
				node.Status.Conditions[0].Status = corev1.ConditionFalse
			} else {
				node.Status.Conditions[0].Status = corev1.ConditionTrue
			}
		}
	}

	// Build the updated state
	for _, nodeName := range nodeNames {
		node := p.nodeMap[nodeName]
		if node == nil {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
				},
			}
		}

		nodePods := p.podStates[nodeName]
		podsByNamespace := make(map[string][]string)
		filteredPodCount := 0
		totalPodCount := len(nodePods)

		for podName, podInfo := range nodePods {
			namespace := strings.Split(podName, "-")[2]

			// Skip if namespace is excluded or not included
			if excludeNamespaces[namespace] || (len(includeNamespaces) > 0 && !includeNamespaces[namespace]) {
				continue
			}

			filteredPodCount++

			indicator := "[green]■[white] "
			switch podInfo.Status {
			case "Failed":
				indicator = "[red]■[white] "
			case "Pending", "Terminating":
				indicator = "[yellow]■[white] "
			}
			podsByNamespace[namespace] = append(podsByNamespace[namespace], indicator)
		}

		for namespace := range podsByNamespace {
			podsByNamespace[namespace] = SortPodIndicators(podsByNamespace[namespace])
		}

		podsByNode[nodeName] = podsByNamespace

		// Only show total if it differs from filtered count
		podCountDisplay := fmt.Sprintf("%d", filteredPodCount)
		if filteredPodCount != totalPodCount {
			podCountDisplay = fmt.Sprintf("%d (%d)", filteredPodCount, totalPodCount)
		}

		nodeData[nodeName] = NodeData{
			Name:          nodeName,
			Status:        "Ready",
			Version:       "v1.24.0",
			PodCount:      podCountDisplay,
			Age:           FormatDuration(time.Since(node.CreationTimestamp.Time)),
			PodIndicators: strings.Join(podsByNode[nodeName]["default"], ""),
			Pods:          nodePods,
			TotalPods:     totalPodCount,
		}
	}

	return nodeData, podsByNode, nil
}
