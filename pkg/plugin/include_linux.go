// nolint // don't complain about this file
package plugin

// Plugins self-register via their init() funcs as long as they are imported.
import (
	_ "github.com/microsoft/retina/pkg/plugin/ciliumeventobserver"
	_ "github.com/microsoft/retina/pkg/plugin/dns"
	_ "github.com/microsoft/retina/pkg/plugin/dropreason"
	_ "github.com/microsoft/retina/pkg/plugin/infiniband"
	_ "github.com/microsoft/retina/pkg/plugin/linuxutil"
	_ "github.com/microsoft/retina/pkg/plugin/mockplugin"
	_ "github.com/microsoft/retina/pkg/plugin/packetforward"
	_ "github.com/microsoft/retina/pkg/plugin/packetparser"
	_ "github.com/microsoft/retina/pkg/plugin/tcpretrans"
)
