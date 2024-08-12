package netpolagent

import (
	"github.com/cilium/cilium/pkg/hive/cell"
)

var Cell = cell.Module(
	"networkpolicy-agent",
	"determines which network policies caused dropped traffic",
	cell.Provide(newNetPolAgent),
)
