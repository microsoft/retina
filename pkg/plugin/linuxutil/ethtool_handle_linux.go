package linuxutil

import (
	"strings"

	"github.com/pkg/errors"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/microsoft/retina/pkg/log"
)

type CachedEthtool struct {
	EthtoolInterface
	unsupported *lru.Cache[string, struct{}]
	l           *log.ZapLogger
}

func NewCachedEthtool(ethHandle EthtoolInterface) *CachedEthtool {
	return &CachedEthtool{
		EthtoolInterface: ethHandle,
		unsupported:      globalLruCache,
		l:                log.Logger().Named(string("EthtoolReader")),
	}
}

var errskip = errors.New("skip interface")

func (ce *CachedEthtool) Stats(intf string) (map[string]uint64, error) {
	// Skip unsupported interfaces
	if _, ok := ce.unsupported.Get(intf); ok {
		return nil, errskip
	}

	ifaceStats, err := ce.EthtoolInterface.Stats(intf)
	if err != nil {
		if strings.Contains(err.Error(), "operation not supported") {
			ce.unsupported.Add(intf, struct{}{})
			return nil, errors.Wrap(err, "interface not supported while retrieving stats")
		}
		return nil, errors.Wrap(err, "failed to retrieve interface stats")
	}
	return ifaceStats, nil
}
