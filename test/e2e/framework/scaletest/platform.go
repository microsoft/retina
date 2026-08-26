package scaletest

import "fmt"

const TrafficOSLabel = "retina.sh/traffic-os"

func SplitCountByNodes(total, linuxNodes, windowsNodes int) (int, int, error) {
	if total < 0 || linuxNodes < 0 || windowsNodes < 0 {
		return 0, 0, fmt.Errorf("counts must not be negative")
	}

	totalNodes := linuxNodes + windowsNodes
	if totalNodes == 0 {
		return 0, 0, fmt.Errorf("at least one node is required")
	}

	windows := total * windowsNodes / totalNodes
	linux := total - windows

	if linuxNodes > 0 && linux == 0 {
		return 0, 0, fmt.Errorf("total %d is too small to allocate a Linux workload", total)
	}
	if windowsNodes > 0 && windows == 0 {
		return 0, 0, fmt.Errorf("total %d is too small to allocate a Windows workload", total)
	}

	return linux, windows, nil
}
