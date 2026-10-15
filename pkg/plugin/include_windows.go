// nolint // don't complain about this file
package plugin

// Plugins self-register via their init() funcs as long as they are imported.
import (
	_ "github.com/microsoft/retina/pkg/plugin/ebpfwindows"
	_ "github.com/microsoft/retina/pkg/plugin/hnsstats"
	_ "github.com/microsoft/retina/pkg/plugin/pktmon"
)
