// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.
package registry

import (
	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/windows/hnsstats"
)

type NewPluginFn func(*kcfg.Config) api.Plugin

var PluginHandler map[api.PluginName]NewPluginFn

func RegisterPlugins() {
	PluginHandler = make(map[api.PluginName]NewPluginFn, 500)
	PluginHandler[hnsstats.Name] = hnsstats.New
}
