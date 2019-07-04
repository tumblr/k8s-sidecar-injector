package coalescer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

var (
	payload          = struct{}{}
	debounceDuration = time.Millisecond * 10
)

func TestCoalesceEvents(t *testing.T) {

	var wg sync.WaitGroup
	input := make(chan interface{})
	fmt.Printf("Starting coalescer window=%s\n", debounceDuration)
	output := Coalesce(context.Background(), debounceDuration, input)
	actualEvents := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(input)
		fmt.Printf("Starting input generator goroutine\n")
		// generate some events on input, then chill out
		fmt.Printf("sending payload\n")
		input <- payload
		fmt.Printf("sending payload\n")
		input <- payload
		fmt.Printf("sending payload\n")
		input <- payload
		time.Sleep(time.Millisecond * 15)
		fmt.Printf("sending payload\n")
		input <- payload
		time.Sleep(time.Millisecond * 15)
		fmt.Printf("sending payload\n")
		input <- payload
		fmt.Printf("Stopping input generator goroutine\n")
	}()

	stop := make(chan struct{})
	go func() {
		// read events from output
		fmt.Printf("Starting output reader goroutine\n")
		for {
			select {
			case <-output:
				//fmt.Printf("output: got event\n")
				actualEvents++
			case <-stop:
				break
			default:
				if output == nil {
					break
				}
			}
		}
		fmt.Printf("Exiting output reader goroutine\n")
	}()
	t.Log("waiting for all routines to finish")

	wg.Wait()
	// wait at least 1 debounce cycle for the final emission of events
	time.Sleep(debounceDuration)
	stop <- struct{}{}
	expectedEvents := 3
	if expectedEvents != actualEvents {
		t.Errorf("expected %d debounced events, but got %d", expectedEvents, actualEvents)
	}
}
