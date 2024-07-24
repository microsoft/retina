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

var watcher *Watcher

// NewWatcher creates a new veth watcher.
func NewWatcher() *Watcher {
	if watcher == nil {
		watcher = &Watcher{
			l: log.Logger().Named(watcherName),
			p: pubsub.New(),
		}
	}
	return watcher
}
