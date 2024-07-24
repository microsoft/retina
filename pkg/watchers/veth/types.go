package veth

import (
	"github.com/microsoft/retina/pkg/log"
	"github.com/microsoft/retina/pkg/pubsub"
)

const (
	watcherName = "veth-watcher"
)

type Watcher struct {
	l *log.ZapLogger
	p pubsub.PubSubInterface
}

// NewWatcher creates a new veth watcher.
func NewWatcher() *Watcher {
	w := &Watcher{
		l: log.Logger().Named(watcherName),
		p: pubsub.New(),
	}

	return w
}
