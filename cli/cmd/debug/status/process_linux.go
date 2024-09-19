package status

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/process"
)

func retinaAgentProcess() (*process.Process, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get processes")
	}
	// look for the process with the name "retina-agent"
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}
		if strings.Contains(name, "retina") {
			return p, nil
		}
	}
	return nil, nil
}
