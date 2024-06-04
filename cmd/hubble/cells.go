// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package hubble

import (
	"github.com/cilium/cilium/pkg/hive/cell"
	"github.com/cilium/proxy/pkg/logging"
	"github.com/cilium/proxy/pkg/logging/logfields"
)

var (
	Agent = cell.Module(
		"agent",
		"Retina-Agent",
		// Infrastructure,
		// ControlPlane,
	)
	daemonSubsys = "daemon"
	logger       = logging.DefaultLogger.WithField(logfields.LogSubsys, daemonSubsys)
)
