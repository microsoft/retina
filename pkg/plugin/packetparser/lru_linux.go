package packetparser

import (
	"time"

	"github.com/cilium/cilium/api/v1/flow"
	"github.com/hashicorp/golang-lru/v2/expirable"
)

type cacheKey struct {
	srcIP   uint32
	dstIP   uint32
	srcPort uint32
	dstPort uint32
	proto   uint8
	dir     uint32
}

var c *expirable.LRU[cacheKey, *flow.Flow]

func cacheInit() {
	c = expirable.NewLRU[cacheKey, *flow.Flow](10000, nil, time.Minute*6)
}
