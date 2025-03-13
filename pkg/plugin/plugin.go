// package plugin aliases types from plugin/registry to prevent import cycles.
package plugin

import "github.com/microsoft/retina/pkg/plugin/registry"

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock/plugin.go -copyright_file=../lib/ignore_headers.txt -package=plugin github.com/microsoft/retina/pkg/plugin Plugin

type (
	Plugin = registry.Plugin
	Func   = registry.PluginFunc
)

func Get(name string) (Func, bool) {
	return registry.Get(name)
}
