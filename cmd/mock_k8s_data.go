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
}

// NewMockK8sDataProvider creates a new MockK8sDataProvider
func NewMockK8sDataProvider() *MockK8sDataProvider {
	return &MockK8sDataProvider{
		nodeMap:     make(map[string]*corev1.Node),
		clusterName: "mock-cluster",
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

func (p *MockK8sDataProvider) UpdateNodeData(includeNamespaces, excludeNamespaces map[string]bool) (map[string]NodeData, map[string]map[string][]string, error) {
	// Initialize random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create mock nodes
	nodeNames := []string{"node1", "node2", "node3"}
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)
	p.nodeMap = make(map[string]*corev1.Node)

	// Mock namespaces
	namespaces := []string{"default", "kube-system", "monitoring"}

	for _, nodeName := range nodeNames {
		// Randomly set node status
		status := corev1.ConditionTrue
		nodeStatus := "Ready"
		if r.Float32() < 0.2 { // 20% chance of not being ready
			status = corev1.ConditionFalse
			nodeStatus = "NotReady"
		}

		// Create mock node with detailed information
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
				Annotations: map[string]string{
					"kubeadm.alpha.kubernetes.io/cri-socket":                 "unix:///var/run/containerd/containerd.sock",
					"node.alpha.kubernetes.io/ttl":                           "0",
					"volumes.kubernetes.io/controller-managed-attach-detach": "true",
				},
			},
			Status: corev1.NodeStatus{
				Conditions: createMockNodeConditions(string(status)),
				NodeInfo: corev1.NodeSystemInfo{
					KubeletVersion:          "v1.24.0",
					KubeProxyVersion:        "v1.24.0",
					OperatingSystem:         "linux",
					Architecture:            "amd64",
					ContainerRuntimeVersion: "containerd://1.6.0",
					OSImage:                 "Ubuntu 20.04.4 LTS",
					KernelVersion:           "5.4.0-109-generic",
				},
				Addresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: fmt.Sprintf("10.0.0.%d", r.Intn(255))},
					{Type: corev1.NodeInternalDNS, Address: nodeName},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("16Gi"),
					corev1.ResourcePods:   resource.MustParse("110"),
				},
			},
		}
		p.nodeMap[nodeName] = node

		// Initialize pod indicators for this node
		podsByNode[nodeName] = make(map[string][]string)

		// Generate pod indicators for each namespace
		totalPods := 0
		for _, ns := range namespaces {
			if excludeNamespaces[ns] {
				continue
			}
			if len(includeNamespaces) > 0 && !includeNamespaces[ns] {
				continue
			}

			podCount := r.Intn(5) + 1 // 1-5 pods per namespace
			indicators := make([]string, podCount)
			for i := 0; i < podCount; i++ {
				// Randomly assign pod status
				rand := r.Float32()
				switch {
				case rand < 0.1: // 10% chance of failure
					indicators[i] = "[red]■[white] "
				case rand < 0.2: // 10% chance of warning
					indicators[i] = "[yellow]■[white] "
				default: // 80% chance of success
					indicators[i] = "[green]■[white] "
				}
			}
			podsByNode[nodeName][ns] = SortPodIndicators(indicators)
			totalPods += podCount
		}

		// Create node data
		nodeData[nodeName] = NodeData{
			Name:     nodeName,
			Status:   nodeStatus,
			Version:  "v1.24.0",
			PodCount: fmt.Sprintf("%d", totalPods),
			Age:      FormatDuration(time.Since(node.CreationTimestamp.Time)),
		}
	}

	return nodeData, podsByNode, nil
}
