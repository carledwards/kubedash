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

// RawNodeData represents the unfiltered data for a node
type RawNodeData struct {
	Node *corev1.Node
	Pods map[string]*corev1.Pod
}

// FilterCriteria defines all possible filtering options
type FilterCriteria struct {
	IncludeNamespaces map[string]bool
	ExcludeNamespaces map[string]bool
	SearchQuery       string
}

// ProcessNodeData handles the common logic for processing node and pod data
func (p *BaseK8sDataProvider) ProcessNodeData(
	nodes []corev1.Node,
	pods []corev1.Pod,
	includeNamespaces, excludeNamespaces map[string]bool,
) (map[string]NodeData, map[string]map[string][]string, error) {
	// First, build the raw data structure
	rawData := make(map[string]RawNodeData)

	// Clear and update node map
	for k := range p.nodeMap {
		delete(p.nodeMap, k)
	}

	// Initialize raw data with nodes
	for i := range nodes {
		node := &nodes[i]
		p.nodeMap[node.Name] = node
		rawData[node.Name] = RawNodeData{
			Node: node,
			Pods: make(map[string]*corev1.Pod),
		}
	}

	// Add all pods to their respective nodes
	for i := range pods {
		pod := &pods[i]
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}
		if nodeData, exists := rawData[nodeName]; exists {
			nodeData.Pods[pod.Name] = pod
			rawData[nodeName] = nodeData
		}
	}

	// Apply filtering criteria
	criteria := FilterCriteria{
		IncludeNamespaces: includeNamespaces,
		ExcludeNamespaces: excludeNamespaces,
		SearchQuery:       "", // Initial load has no search query
	}

	return p.filterAndTransformData(rawData, criteria)
}

// filterAndTransformData converts raw data into filtered view data
func (p *BaseK8sDataProvider) filterAndTransformData(
	rawData map[string]RawNodeData,
	criteria FilterCriteria,
) (map[string]NodeData, map[string]map[string][]string, error) {
	nodeData := make(map[string]NodeData)
	podsByNode := make(map[string]map[string][]string)

	for nodeName, raw := range rawData {
		// Initialize node status
		nodeStatus := NodeStatusNotReady
		for _, condition := range raw.Node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				if condition.Status == corev1.ConditionTrue {
					nodeStatus = NodeStatusReady
				}
				break
			}
		}

		// Create base node data
		data := NodeData{
			Name:      nodeName,
			Status:    nodeStatus,
			Version:   raw.Node.Status.NodeInfo.KubeletVersion,
			Age:       FormatDuration(time.Since(raw.Node.CreationTimestamp.Time)),
			Pods:      make(map[string]PodInfo),
			TotalPods: len(raw.Pods), // Store total unfiltered count
		}

		// Initialize pod indicators structure
		podsByNode[nodeName] = make(map[string][]string)

		// Filter and process pods
		filteredPodCount := 0
		for podName, pod := range raw.Pods {
			// Apply namespace filters
			if criteria.ExcludeNamespaces[pod.Namespace] {
				continue
			}
			if len(criteria.IncludeNamespaces) > 0 && !criteria.IncludeNamespaces[pod.Namespace] {
				continue
			}

			// Apply search filter if present
			if criteria.SearchQuery != "" {
				if !strings.Contains(strings.ToLower(podName), strings.ToLower(criteria.SearchQuery)) {
					continue
				}
			}

			// Pod passed all filters, include it
			filteredPodCount++
			data.Pods[podName] = GetPodInfo(pod)

			// Add pod indicator
			if _, exists := podsByNode[nodeName][pod.Namespace]; !exists {
				podsByNode[nodeName][pod.Namespace] = make([]string, 0)
			}
			podsByNode[nodeName][pod.Namespace] = append(
				podsByNode[nodeName][pod.Namespace],
				GetPodIndicator(pod),
			)
		}

		// Set pod count display
		if filteredPodCount == data.TotalPods {
			data.PodCount = fmt.Sprintf("%d", filteredPodCount)
		} else {
			data.PodCount = fmt.Sprintf("%d (%d)", filteredPodCount, data.TotalPods)
		}

		// Sort pod indicators for consistent display
		for ns := range podsByNode[nodeName] {
			podsByNode[nodeName][ns] = SortPodIndicators(podsByNode[nodeName][ns])
		}

		// Set pod indicators string
		var indicators []string
		for _, nsIndicators := range podsByNode[nodeName] {
			indicators = append(indicators, strings.Join(nsIndicators, ""))
		}
		data.PodIndicators = strings.Join(indicators, "")

		nodeData[nodeName] = data
	}

	return nodeData, podsByNode, nil
}
