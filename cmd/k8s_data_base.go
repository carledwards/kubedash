package cmd

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// BaseK8sDataProvider provides common functionality for both real and mock K8s data providers
type BaseK8sDataProvider struct {
	nodeMap map[string]*corev1.Node
}

// GetNodeMap implements part of K8sDataProvider interface
func (p *BaseK8sDataProvider) GetNodeMap() map[string]*corev1.Node {
	return p.nodeMap
}

// ProcessNodeData handles the common logic for processing node and pod data
func (p *BaseK8sDataProvider) ProcessNodeData(
	nodes []corev1.Node,
	pods []corev1.Pod,
	includeNamespaces, excludeNamespaces map[string]bool,
) (map[string]NodeData, map[string]map[string][]string, error) {
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)

	// Clear and update node map
	for k := range p.nodeMap {
		delete(p.nodeMap, k)
	}
	for i := range nodes {
		node := &nodes[i]
		p.nodeMap[node.Name] = node
	}

	// Initialize node data structures and total pod counts
	nodeTotalPods := make(map[string]int)
	for _, node := range nodes {
		nodeStatus := NodeStatusNotReady
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					nodeStatus = NodeStatusReady
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
			TotalPods:     0,
		}
		nodeData[node.Name] = data
		podsByNode[node.Name] = make(map[string][]string)
		nodeTotalPods[node.Name] = 0
	}

	// First pass: count total pods per node
	for _, pod := range pods {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}
		nodeTotalPods[nodeName]++
	}

	// Second pass: process filtered pods and update counts
	for _, pod := range pods {
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

		// Get pod indicator
		indicator := GetPodIndicator(&pod)
		podsByNode[nodeName][ns] = append(podsByNode[nodeName][ns], indicator)

		// Update pod info in node data
		if data, exists := nodeData[nodeName]; exists {
			podInfo := GetPodInfo(&pod)
			data.Pods[pod.Name] = podInfo
			data.TotalPods = nodeTotalPods[nodeName]

			// Only show total if it differs from filtered count
			if len(data.Pods) == data.TotalPods {
				data.PodCount = fmt.Sprintf("%d", len(data.Pods))
			} else {
				data.PodCount = fmt.Sprintf("%d (%d)", len(data.Pods), data.TotalPods)
			}

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
