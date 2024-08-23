package linuxutil

import (
	"github.com/pkg/errors"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/microsoft/retina/pkg/log"
	"go.uber.org/zap"
)

type CachedEthtool struct {
	ethHandle   EthtoolInterface
	unsupported *lru.Cache[string, struct{}]
	l           *log.ZapLogger
}

func NewCachedEthtool(ethHandle EthtoolInterface, opts *EthtoolOpts) *CachedEthtool {
	cache, err := lru.New[string, struct{}](int(opts.limit))
	if err != nil {
		log.Logger().Error("failed to create LRU cache: ", zap.Error(err))
	}

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
		ce.unsupported.Add(intf, struct{}{})
		return nil, errors.Wrap(err, "error while getting interface stats")
	}
	return ifaceStats, nil
}

func (ce *CachedEthtool) Close() {
	ce.ethHandle.Close()
}
