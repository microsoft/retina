package linuxutil

import (
	"errors"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type CachedEthtool struct {
	ethHandle   EthtoolInterface
	unsupported *lru.Cache[string, any]
	l           *log.ZapLogger
}

func NewCachedEthtool(ethHandle EthtoolInterface, opts *EthtoolOpts) *CachedEthtool {
	cache, err := lru.New[string, any](int(opts.limit))
	if err != nil {
		log.Logger().Error("failed to create LRU cache: ", zap.Error(err))
	}
	//not sure if I should do the same way to process the handle
	return &CachedEthtool{
		ethHandle:   ethHandle,
		unsupported: cache,
		l:           log.Logger().Named(string("EthtoolReader")),
	}
}

var errskip = errors.New("skip interface")

func (ce *CachedEthtool) Stats(intf string) (map[string]uint64, error) {
	// Skip unsupported interfaces
	if _, ok := ce.unsupported.Get(intf); ok {
		return nil, errskip
	}

	ifaceStats, err := ce.ethHandle.Stats(intf)

	if err != nil {
		ce.unsupported.Add(intf, nil)
		return nil, err
	}
	return ifaceStats, nil
}

func (ce *CachedEthtool) Close() {
	ce.ethHandle.Close()
}
