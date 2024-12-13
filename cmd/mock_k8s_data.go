package cmd

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockK8sDataProvider implements K8sProvider using mock data
type MockK8sDataProvider struct {
	BaseK8sDataProvider
	clusterName string
	podStates   map[string]map[string]PodInfo
	rand        *rand.Rand // node -> pod name -> pod info
	nodeCounter int        // Counter for generating new node names
}

// NewMockK8sDataProvider creates a new MockK8sDataProvider
func NewMockK8sDataProvider() *MockK8sDataProvider {
	provider := &MockK8sDataProvider{
		BaseK8sDataProvider: BaseK8sDataProvider{
			nodeMap: make(map[string]*corev1.Node),
		},
		clusterName: "mock-cluster",
		podStates:   make(map[string]map[string]PodInfo),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
		nodeCounter: 3, // Start with 3 initial nodes
	}

	// Initialize with some default nodes
	for i := 1; i <= 3; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		provider.nodeMap[nodeName] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: nodeName,
				CreationTimestamp: metav1.Time{
					Time: time.Now().Add(-24 * time.Hour), // Created 24h ago
				},
			},
			Status: corev1.NodeStatus{
				Conditions: createMockNodeConditions("True"),
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion: "v1.24.0",
				},
			},
		}

		// Add some default pods to each node
		provider.podStates[nodeName] = make(map[string]PodInfo)

		// Add a monitoring pod
		monitoringPod := fmt.Sprintf("%s-pod-monitoring-1", nodeName)
		provider.podStates[nodeName][monitoringPod] = PodInfo{
			Name:         monitoringPod,
			Status:       PodStatusRunning,
			RestartCount: 0,
			ContainerInfo: map[string]ContainerInfo{
				"prometheus": {Status: PodStatusRunning, RestartCount: 0},
			},
		}

		// Add a default namespace pod
		defaultPod := fmt.Sprintf("%s-pod-default-1", nodeName)
		provider.podStates[nodeName][defaultPod] = PodInfo{
			Name:         defaultPod,
			Status:       PodStatusRunning,
			RestartCount: 2,
			ContainerInfo: map[string]ContainerInfo{
				"web-server": {Status: PodStatusRunning, RestartCount: 2},
				"sidecar":    {Status: PodStatusRunning, RestartCount: 0},
			},
		}

		// Add a kube-system pod
		systemPod := fmt.Sprintf("%s-pod-kube-system-1", nodeName)
		provider.podStates[nodeName][systemPod] = PodInfo{
			Name:         systemPod,
			Status:       PodStatusRunning,
			RestartCount: 1,
			ContainerInfo: map[string]ContainerInfo{
				"kube-proxy": {Status: PodStatusRunning, RestartCount: 1},
			},
		}
	}

	return provider
}

func (p *MockK8sDataProvider) GetClusterName() string {
	return p.clusterName
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
	statuses := []string{PodStatusRunning, PodStatusPending, "Failed", PodStatusTerminating}
	status := statuses[r.Intn(len(statuses))]

	// Create container info
	containers := make(map[string]ContainerInfo)
	containerCount := r.Intn(2) + 1 // 1-2 containers per pod

	totalRestarts := 0
	for i := 0; i < containerCount; i++ {
		containerName := fmt.Sprintf("%s-container-%d", podName, i)
		restarts := r.Intn(5) // 0-4 restarts
		totalRestarts += restarts
		containers[containerName] = ContainerInfo{
			Status:       status,
			RestartCount: restarts,
		}
	}

	return PodInfo{
		Name:          podName,
		Status:        status,
		RestartCount:  totalRestarts,
		ContainerInfo: containers,
	}
}

func (p *MockK8sDataProvider) GetPodsByNode(includeNamespaces, excludeNamespaces map[string]bool) (map[string]map[string]PodInfo, error) {
	result := make(map[string]map[string]PodInfo)

	// Copy the existing pod states
	for nodeName, pods := range p.podStates {
		nodePods := make(map[string]PodInfo)
		for podName, podInfo := range pods {
			// Extract namespace from pod name (mock-specific format)
			namespace := "default"
			if parts := strings.Split(podName, "-"); len(parts) > 2 {
				namespace = parts[2]
			}

			// Apply namespace filtering
			if excludeNamespaces[namespace] {
				continue
			}
			if len(includeNamespaces) > 0 && !includeNamespaces[namespace] {
				continue
			}

			nodePods[podName] = podInfo
		}
		if len(nodePods) > 0 {
			result[nodeName] = nodePods
		}
	}

	return result, nil
}

func (p *MockK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	r := p.rand

	// Get list of current nodes
	nodeNames := make([]string, 0)
	for nodeName := range p.nodeMap {
		nodeNames = append(nodeNames, nodeName)
	}

	// Randomly pick a change type
	// 0 = pod addition
	// 1 = pod status change
	// 2 = node readiness change
	// 3 = restart count change
	// 4 = add new node
	// 5 = delete node
	// 6 = delete pod
	changeType := r.Intn(7)

	// Process changes based on type
	switch changeType {
	case 0: // Add a new pod
		if len(nodeNames) > 0 {
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			namespace := []string{"default", "kube-system", "monitoring"}[r.Intn(3)]
			if !excludeNamespaces[namespace] && (len(includeNamespaces) == 0 || includeNamespaces[namespace]) {
				podName := fmt.Sprintf("%s-pod-%s-%d", randomNode, namespace, len(p.podStates[randomNode])+1)
				podInfo := createMockPodInfo(r, podName)

				if _, exists := p.podStates[randomNode]; !exists {
					p.podStates[randomNode] = make(map[string]PodInfo)
				}
				p.podStates[randomNode][podName] = podInfo
			}
		}

	case 1: // Update a pod status
		if len(nodeNames) > 0 {
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			if len(p.podStates[randomNode]) > 0 {
				podKeys := make([]string, 0, len(p.podStates[randomNode]))
				for podName := range p.podStates[randomNode] {
					podKeys = append(podKeys, podName)
				}
				randomPod := podKeys[r.Intn(len(podKeys))]
				updatedPod := createMockPodInfo(r, randomPod)
				p.podStates[randomNode][randomPod] = updatedPod
			}
		}

	case 2: // Change node readiness
		if len(nodeNames) > 0 {
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			node := p.nodeMap[randomNode]
			if len(node.Status.Conditions) > 0 {
				if node.Status.Conditions[0].Status == corev1.ConditionTrue {
					node.Status.Conditions[0].Status = corev1.ConditionFalse
				} else {
					node.Status.Conditions[0].Status = corev1.ConditionTrue
				}
			}
		}

	case 3: // Increment restart count for a random container
		if len(nodeNames) > 0 {
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			if len(p.podStates[randomNode]) > 0 {
				podKeys := make([]string, 0, len(p.podStates[randomNode]))
				for podName := range p.podStates[randomNode] {
					podKeys = append(podKeys, podName)
				}
				randomPod := podKeys[r.Intn(len(podKeys))]
				podInfo := p.podStates[randomNode][randomPod]

				// Get a random container
				containerKeys := make([]string, 0, len(podInfo.ContainerInfo))
				for containerName := range podInfo.ContainerInfo {
					containerKeys = append(containerKeys, containerName)
				}
				if len(containerKeys) > 0 {
					randomContainer := containerKeys[r.Intn(len(containerKeys))]
					containerInfo := podInfo.ContainerInfo[randomContainer]

					// Increment restart count
					containerInfo.RestartCount++
					podInfo.ContainerInfo[randomContainer] = containerInfo

					// Update total pod restart count
					podInfo.RestartCount++
					p.podStates[randomNode][randomPod] = podInfo
				}
			}
		}

	case 4: // Add new node
		p.nodeCounter++
		newNodeName := fmt.Sprintf("node%d", p.nodeCounter)
		p.nodeMap[newNodeName] = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: newNodeName,
				CreationTimestamp: metav1.Time{
					Time: time.Now(),
				},
			},
			Status: corev1.NodeStatus{
				Conditions: createMockNodeConditions("True"),
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion: "v1.24.0",
				},
			},
		}
		nodeNames = append(nodeNames, newNodeName)

	case 5: // Delete node
		if len(nodeNames) > 1 { // Keep at least one node
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			delete(p.nodeMap, randomNode)
			delete(p.podStates, randomNode)
		}

	case 6: // Delete pod
		if len(nodeNames) > 0 {
			randomNode := nodeNames[r.Intn(len(nodeNames))]
			if len(p.podStates[randomNode]) > 0 {
				podKeys := make([]string, 0, len(p.podStates[randomNode]))
				for podName := range p.podStates[randomNode] {
					podKeys = append(podKeys, podName)
				}
				randomPod := podKeys[r.Intn(len(podKeys))]
				delete(p.podStates[randomNode], randomPod)
			}
		}
	}

	// Build pods list from pod states
	pods := make([]corev1.Pod, 0)
	for nodeName, nodePods := range p.podStates {
		for podName, podInfo := range nodePods {
			namespace := "default"
			if parts := strings.Split(podName, "-"); len(parts) > 2 {
				namespace = parts[2]
			}

			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					NodeName: nodeName,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPhase(podInfo.Status),
				},
			}
			pods = append(pods, pod)
		}
	}

	// Convert nodeMap to slice for processing
	nodes := make([]corev1.Node, 0, len(p.nodeMap))
	for _, node := range p.nodeMap {
		nodes = append(nodes, *node)
	}

	return p.ProcessNodeData(nodes, pods, includeNamespaces, excludeNamespaces)
}
