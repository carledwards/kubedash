package cmd

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
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

// GetPodIndicator returns a pod's status indicator with color
func GetPodIndicator(pod *corev1.Pod) string {
	// Check if pod is in a failed phase
	if pod.Status.Phase == "Failed" {
		return "[red]■[white] "
	}

	// Check if pod is pending
	if pod.Status.Phase == "Pending" {
		return "[yellow]■[white] "
	}

	// Check container statuses
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// Check if container is not ready
		if !containerStatus.Ready {
			// Check if container is in a waiting state
			if containerStatus.State.Waiting != nil {
				reason := containerStatus.State.Waiting.Reason
				// Common error reasons that should show as red
				if reason == "CrashLoopBackOff" || reason == "Error" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					return "[red]■[white] "
				}
				// Other waiting states show as yellow
				return "[yellow]■[white] "
			}
			// Container not ready but not waiting - show as yellow
			return "[yellow]■[white] "
		}

		// Check for restarts
		if containerStatus.RestartCount > 0 {
			return "[yellow]■[white] "
		}

		// Check container state
		if containerStatus.State.Waiting != nil {
			return "[yellow]■[white] "
		}
		if containerStatus.State.Terminated != nil {
			return "[red]■[white] "
		}
	}

	// If we have no container statuses but pod is running, show yellow
	if len(pod.Status.ContainerStatuses) == 0 && pod.Status.Phase == "Running" {
		return "[yellow]■[white] "
	}

	// All containers are ready and no issues
	return "[green]■[white] "
}
