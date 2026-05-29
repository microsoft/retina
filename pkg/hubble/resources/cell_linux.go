package resources

import (
	"github.com/cilium/cilium/pkg/k8s"
	"github.com/cilium/hive/cell"
)

var Cell = cell.Module(
	"resources",
	"Resources for Hubble",
	cell.Provide(NewServiceHandler),
	cell.Provide(
		k8s.CiliumIdentityResource,
		NewCiliumIdentityHandler,
	),
)
