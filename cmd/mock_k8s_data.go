package cmd

import (
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockK8sDataProvider implements K8sDataProvider using mock data
type MockK8sDataProvider struct {
	nodeMap     map[string]*corev1.Node
	clusterName string
	podStates   map[string]map[string]PodInfo // node -> pod name -> pod info
}

// NewMockK8sDataProvider creates a new MockK8sDataProvider
func NewMockK8sDataProvider() *MockK8sDataProvider {
	return &MockK8sDataProvider{
		nodeMap:     make(map[string]*corev1.Node),
		clusterName: "mock-cluster",
		podStates:   make(map[string]map[string]PodInfo),
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
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	nodeNames := []string{"node1", "node2", "node3"}
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)
	p.nodeMap = make(map[string]*corev1.Node)

	namespaces := []string{"default", "kube-system", "monitoring"}

	for _, nodeName := range nodeNames {
		status := corev1.ConditionTrue
		nodeStatus := "Ready"
		if r.Float32() < 0.2 {
			status = corev1.ConditionFalse
			nodeStatus = "NotReady"
		}

		node := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:              nodeName,
				CreationTimestamp: metav1.Time{Time: time.Now().Add(-24 * time.Hour * time.Duration(r.Intn(30)))},
				Labels: map[string]string{
					"kubernetes.io/hostname":                nodeName,
					"node-role.kubernetes.io/control-plane": "",
					"beta.kubernetes.io/arch":               "amd64",
					"beta.kubernetes.io/os":                 "linux",
				},
			},
			Status: corev1.NodeStatus{
				Conditions:  createMockNodeConditions(string(status)),
				NodeInfo:    corev1.NodeSystemInfo{KubeletVersion: "v1.24.0"},
				Addresses:   []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: fmt.Sprintf("10.0.0.%d", r.Intn(255))}},
				Capacity:    corev1.ResourceList{corev1.ResourcePods: resource.MustParse("110")},
				Allocatable: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("110")},
			},
		}
		p.nodeMap[nodeName] = node

		podsByNode[nodeName] = make(map[string][]string)
		if _, exists := p.podStates[nodeName]; !exists {
			p.podStates[nodeName] = make(map[string]PodInfo)
		}

		totalPods := 0
		nodePods := make(map[string]PodInfo)

		for _, ns := range namespaces {
			if excludeNamespaces[ns] {
				continue
			}
			if len(includeNamespaces) > 0 && !includeNamespaces[ns] {
				continue
			}

			podCount := r.Intn(5) + 1
			indicators := make([]string, podCount)

			for i := 0; i < podCount; i++ {
				podName := fmt.Sprintf("%s-pod-%s-%d", nodeName, ns, i)

				// Either use existing pod state or create new one
				var podInfo PodInfo
				if existingPod, exists := p.podStates[nodeName][podName]; exists && r.Float32() < 0.8 {
					// 80% chance to keep existing pod state
					podInfo = existingPod
				} else {
					podInfo = createMockPodInfo(r, podName)
				}

				nodePods[podName] = podInfo

				// Set indicator based on pod status
				switch podInfo.Status {
				case "Failed":
					indicators[i] = "[red]■[white] "
				case "Pending", "Terminating":
					indicators[i] = "[yellow]■[white] "
				default:
					indicators[i] = "[green]■[white] "
				}
			}

			podsByNode[nodeName][ns] = SortPodIndicators(indicators)
			totalPods += podCount
		}

		// Update pod states for this node
		p.podStates[nodeName] = nodePods

		// Create node data with pod information
		nodeData[nodeName] = NodeData{
			Name:          nodeName,
			Status:        nodeStatus,
			Version:       "v1.24.0",
			PodCount:      fmt.Sprintf("%d", totalPods),
			Age:           FormatDuration(time.Since(node.CreationTimestamp.Time)),
			PodIndicators: fmt.Sprintf("%d pods", totalPods),
			Pods:          nodePods,
		}
	}

	return nodeData, podsByNode, nil
}
