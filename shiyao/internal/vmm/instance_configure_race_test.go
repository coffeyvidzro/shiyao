package vmm

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type blockingNetworkLease struct {
	cfg       network.Config
	cid       uint32
	setupOnce sync.Once
	setup     chan struct{}
	release   chan struct{}
}

func (l *blockingNetworkLease) Config() network.Config { return l.cfg }
func (l *blockingNetworkLease) CID() uint32             { return l.cid }
func (l *blockingNetworkLease) Setup(context.Context) error {
	l.setupOnce.Do(func() { close(l.setup) })
	<-l.release
	return errors.New("setup interrupted for test")
}
func (l *blockingNetworkLease) Release(context.Context) error { return nil }
