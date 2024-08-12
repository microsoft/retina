// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

// Package registry contains the plugin registry for Retina. It is used for plugin registration and instantiation.
package registry

import (
	kcfg "github.com/microsoft/retina/pkg/config"

	"github.com/microsoft/retina/pkg/plugin/api"
	"github.com/microsoft/retina/pkg/plugin/ciliumeventobserver"
	"github.com/microsoft/retina/pkg/plugin/dns"
	"github.com/microsoft/retina/pkg/plugin/dropreason"
	"github.com/microsoft/retina/pkg/plugin/infiniband"
	"github.com/microsoft/retina/pkg/plugin/linuxutil"
	"github.com/microsoft/retina/pkg/plugin/mockplugin"
	"github.com/microsoft/retina/pkg/plugin/packetforward"
	"github.com/microsoft/retina/pkg/plugin/packetparser"
	"github.com/microsoft/retina/pkg/plugin/tcpretrans"
)

type NewPluginFn func(*kcfg.Config) api.Plugin

var PluginHandler map[api.PluginName]NewPluginFn

func RegisterPlugins() {
	PluginHandler = make(map[api.PluginName]NewPluginFn, 500)
	PluginHandler[dropreason.Name] = dropreason.New
	PluginHandler[packetforward.Name] = packetforward.New
	PluginHandler[linuxutil.Name] = linuxutil.New
	PluginHandler[infiniband.Name] = infiniband.New
	PluginHandler[packetparser.Name] = packetparser.New
	PluginHandler[dns.Name] = dns.New
	PluginHandler[tcpretrans.Name] = tcpretrans.New
	PluginHandler[mockplugin.Name] = mockplugin.New
	PluginHandler[ciliumeventobserver.Name] = ciliumeventobserver.New
}
