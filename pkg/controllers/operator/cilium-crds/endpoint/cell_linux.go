package endpointcontroller

import "github.com/cilium/hive/cell"

var Cell = cell.Module(
	"endpointcontroller",
	"controller for cilium endpoint and identity creation and updates",
	cell.Invoke(registerEndpointController),
)
