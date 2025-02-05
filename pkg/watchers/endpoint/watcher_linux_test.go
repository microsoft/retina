package endpoint

import (
	"context"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/microsoft/retina/pkg/log"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestName(t *testing.T) {
	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)
	w := NewWatcher()
	require.Equal(t, watcherName, w.Name())
}

func TestStart(t *testing.T) {
	// Monkey patch netlink.LinkSubscribeWithOptions with our fakeLinkSubscribe function to simulate netlink events.
	// TIL: https://bou.ke/blog/monkey-patching-in-go/
	// Should be fine for testing purposes (?)
	patch := monkey.Patch(netlink.LinkSubscribeWithOptions, fakeLinkSubscribe)
	defer patch.Unpatch()

	_, err := log.SetupZapLogger(log.GetDefaultLogOpts())
	require.NoError(t, err)

	w := NewWatcher()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- w.Start(ctx)
	}()

	// Wait briefly to allow the fake event to be sent.
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to stop the watcher.
	cancel()

	// Wait for Start to finish.
	err = <-doneCh
	require.NoError(t, err, "Start should exit without error")
}

// fakeLinkSubscribe simulates netlink events by sending a fake event after a short delay.
func fakeLinkSubscribe(netlinkEvCh chan<- netlink.LinkUpdate, done <-chan struct{}, _ netlink.LinkSubscribeOptions) error {
	go func() {
		time.Sleep(10 * time.Millisecond)
		fakeVethCreatedEvent := netlink.LinkUpdate{
			Header: unix.NlMsghdr{
				Type: unix.RTM_NEWLINK,
			},
			Link: &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name:      "veth0",
					Index:     1,
					OperState: netlink.OperUp,
				},
			},
		}
		netlinkEvCh <- fakeVethCreatedEvent
		fakeVethDeletedEvent := netlink.LinkUpdate{
			Header: unix.NlMsghdr{
				Type: unix.RTM_DELLINK,
			},
			Link: &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name:      "veth0",
					Index:     1,
					OperState: netlink.OperDown,
				},
			},
		}
		netlinkEvCh <- fakeVethDeletedEvent
		<-done
	}()
	return nil
}
