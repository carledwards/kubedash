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
	BaseK8sDataProvider
	clusterName string
	podStates   map[string]map[string]PodInfo
	rand        *rand.Rand // node -> pod name -> pod info
}

// NewMockK8sDataProvider creates a new MockK8sDataProvider
func NewMockK8sDataProvider() *MockK8sDataProvider {
	return &MockK8sDataProvider{
		BaseK8sDataProvider: BaseK8sDataProvider{
			nodeMap: make(map[string]*corev1.Node),
		},
		clusterName: "mock-cluster",
		podStates:   make(map[string]map[string]PodInfo),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
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
	statuses := []string{"Running", "Pending", "Failed", "Terminating"}
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

func (p *MockK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	r := p.rand

	// Randomly pick a node to change
	nodeNames := []string{"node1", "node2", "node3"}
	randomNode := nodeNames[r.Intn(len(nodeNames))]

	// Simulate a single change
	changeType := r.Intn(4) // 0 = pod addition, 1 = pod status change, 2 = node readiness change, 3 = restart count change

	// Create or update mock nodes and pods
	nodes := make([]corev1.Node, 0, len(nodeNames))
	pods := make([]corev1.Pod, 0)

	// Process changes based on type
	switch changeType {
	case 0: // Add a new pod
		namespace := []string{"default", "kube-system", "monitoring"}[r.Intn(3)]
		if !excludeNamespaces[namespace] && (len(includeNamespaces) == 0 || includeNamespaces[namespace]) {
			podName := fmt.Sprintf("%s-pod-%s-%d", randomNode, namespace, len(p.podStates[randomNode])+1)
			podInfo := createMockPodInfo(r, podName)

			if _, exists := p.podStates[randomNode]; !exists {
				p.podStates[randomNode] = make(map[string]PodInfo)
			}
			p.podStates[randomNode][podName] = podInfo
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
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: randomNode,
				},
				Status: corev1.NodeStatus{
					Conditions: createMockNodeConditions("False"),
				},
			}
			p.nodeMap[randomNode] = node
		}

		if len(node.Status.Conditions) > 0 {
			if node.Status.Conditions[0].Status == corev1.ConditionTrue {
				node.Status.Conditions[0].Status = corev1.ConditionFalse
			} else {
				node.Status.Conditions[0].Status = corev1.ConditionTrue
			}
		}

	case 3: // Increment restart count for a random container
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

	// Build nodes list
	for _, nodeName := range nodeNames {
		node, exists := p.nodeMap[nodeName]
		if !exists {
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					CreationTimestamp: metav1.Time{
						Time: time.Now().Add(-24 * time.Hour), // Mock nodes created 24h ago
					},
				},
				Status: corev1.NodeStatus{
					Conditions: createMockNodeConditions("True"),
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.24.0",
					},
				},
			}
			p.nodeMap[nodeName] = node
		}
		nodes = append(nodes, *node)
	}

	// Build pods list from pod states
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

	return p.ProcessNodeData(nodes, pods, includeNamespaces, excludeNamespaces)
}
