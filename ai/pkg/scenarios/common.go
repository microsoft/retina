package scenarios

import "regexp"

// common parameters
var (
	k8sNameRegex = regexp.MustCompile(`^[a-zA-Z][-a-zA-Z0-9]*$`)
	nodesRegex   = regexp.MustCompile(`^\[[a-zA-Z][-a-zA-Z0-9_,]*\]$`)

	Namespace1 = &ParameterSpec{
		Name:        "namespace1",
		DataType:    "string",
		Description: "Namespace 1",
		Optional:    false,
		Regex:       k8sNameRegex,
	}

	PodPrefix1 = &ParameterSpec{
		Name:        "podPrefix1",
		DataType:    "string",
		Description: "Pod prefix 1",
		Optional:    true,
		Regex:       k8sNameRegex,
	}

	Namespace2 = &ParameterSpec{
		Name:        "namespace2",
		DataType:    "string",
		Description: "Namespace 2",
		Optional:    true,
		Regex:       k8sNameRegex,
	}

	PodPrefix2 = &ParameterSpec{
		Name:        "podPrefix2",
		DataType:    "string",
		Description: "Pod prefix 2",
		Optional:    true,
		Regex:       k8sNameRegex,
	}

	Nodes = &ParameterSpec{
		Name:        "nodes",
		DataType:    "[]string",
		Description: "Nodes",
		Optional:    true,
		Regex:       nodesRegex,
	}
)
