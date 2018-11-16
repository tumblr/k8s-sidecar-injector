package coalescer

import (
	"context"
	"time"

	"github.com/golang/glog"
)

// Coalesce takes an input chan, and coalesced inputs with a timebound of interval, after which
// it signals on output chan with the last value from input chan
func Coalesce(ctx context.Context, interval time.Duration, input chan interface{}) <-chan interface{} {
	output := make(chan interface{})
	go func() {
		var (
			signalled bool
			inputOpen = true // assume input chan is open before we run our select loop
		)
		glog.V(2).Infof("debouncing reconciliation signals with window %s", interval.String())
		for {
			doneCh := ctx.Done()
			select {
			case <-doneCh:
				if signalled {
					output <- struct{}{}
				}
				return
			case <-time.After(interval):
				if signalled {
					glog.V(5).Infof("signalling reconciliation after %s", interval.String())
					output <- struct{}{}
					signalled = false
				}
			case _, inputOpen = <-input:
				if inputOpen { // only record events if the input channel is still open
					glog.V(4).Infof("got reconciliation signal, debouncing for %s", interval.String())
					signalled = true
				}
			}
			// stop running the Coalescer only when all input+output channels are closed!
			if !inputOpen {
				// input is closed, so lets signal one last time if we have any pending unsignalled events
				if signalled {
					// send final event, so we dont miss the trailing event after input chan close
					output <- struct{}{}
				}
				glog.V(1).Infof("coalesce routine terminated, input channel is closed")
				return
			}
		}
	}()
	return output
}
