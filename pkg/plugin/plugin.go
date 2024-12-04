// package plugin aliases types from plugin/registry to prevent import cycles.
package plugin

import "github.com/microsoft/retina/pkg/plugin/registry"

type Plugin = registry.Plugin

//go:generate go run go.uber.org/mock/mockgen@v0.4.0 -destination=mock/plugin.go -copyright_file=../lib/ignore_headers.txt -package=plugin github.com/microsoft/retina/pkg/plugin Plugin
var Registry = registry.Plugins
