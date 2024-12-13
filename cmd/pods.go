package cmd

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// PodInfo represents information about a pod and its containers
type PodInfo struct {
	Name          string
	Status        string
	RestartCount  int
	ContainerInfo map[string]ContainerInfo
}

// ContainerInfo represents information about a container
type ContainerInfo struct {
	Status       string
	RestartCount int
}

// GetPodInfo extracts PodInfo from a Kubernetes Pod
func GetPodInfo(pod *corev1.Pod) PodInfo {
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

		status := PodStatusUnknown
		restartCount := 0
		if containerStatus != nil {
			if containerStatus.State.Running != nil {
				status = PodStatusRunning
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
		podInfo.Status = PodStatusTerminating
	}

	return podInfo
}

// GetPodIndicator returns a visual indicator for pod status
func GetPodIndicator(pod *corev1.Pod) string {
	// First check for restarts
	var totalRestarts int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		totalRestarts += containerStatus.RestartCount
	}

	if totalRestarts > 0 {
		return PodIndicatorYellow
	}

	switch {
	case pod.Status.Phase == corev1.PodRunning:
		return PodIndicatorGreen
	case pod.Status.Phase == corev1.PodPending:
		return PodIndicatorYellow
	default:
		return PodIndicatorRed
	}
}

// SortPodIndicators sorts pod indicators by color (RED, YELLOW, GREEN)
func SortPodIndicators(indicators []string) []string {
	// Define color priority (red = 0, yellow = 1, green = 2)
	colorPriority := map[string]int{
		ColorRed:    0,
		ColorYellow: 1,
		ColorGreen:  2,
	}

	// Sort indicators by color
	sorted := make([]string, len(indicators))
	copy(sorted, indicators)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			var color1, color2 string
			if strings.Contains(sorted[i], ColorTagRed) {
				color1 = ColorRed
			} else if strings.Contains(sorted[i], ColorTagYellow) {
				color1 = ColorYellow
			} else {
				color1 = ColorGreen
			}

			if strings.Contains(sorted[j], ColorTagRed) {
				color2 = ColorRed
			} else if strings.Contains(sorted[j], ColorTagYellow) {
				color2 = ColorYellow
			} else {
				color2 = ColorGreen
			}

			if colorPriority[color1] > colorPriority[color2] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}
