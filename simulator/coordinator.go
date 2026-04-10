// Package simulator handles coordinating multiple parallel auctions.
package simulator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SimulationSummary is the top-level report returned after all auctions finish.
type SimulationSummary struct {
	TotalAuctions  int
	Results        []AuctionResult
	FirstStartedAt time.Time
	LastEndedAt    time.Time
	TotalDuration  time.Duration // lastEnded − firstStarted
	WriteErrors    []error       // non-fatal file-write errors
}

// Run starts all auctions, waits for completion, writes files, and returns a summary.
func Run(ctx context.Context, governor *ResourceGovernor) SimulationSummary {
	// Buffer results up to TotalAuctions.
	resultCh := make(chan AuctionResult, TotalAuctions)

	// Timing sentinels using atomic operations.
	var (
		firstStarted atomic.Int64 // UnixNano of first auction start
		lastEnded    atomic.Int64 // UnixNano of last auction end
	)

	// WaitGroup tracks all auction goroutines.
	var wg sync.WaitGroup
	wg.Add(TotalAuctions)

	// Launch all auction goroutines.

	for i := 1; i <= TotalAuctions; i++ {
		auctionID := i // capture for goroutine

		// Acquire semaphore slot before spawning goroutine.
		governor.Acquire()

		go func() {
			defer wg.Done()
			defer governor.Release()

			result := RunAuction(ctx, auctionID)

			// Update timings atomically.
			recordFirst(&firstStarted, result.StartedAt.UnixNano())
			recordLast(&lastEnded, result.ClosedAt.UnixNano())

			resultCh <- result
		}()
	}

	// Close resultCh once every auction goroutine has finished.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results & write files in parallel.

	results := make([]AuctionResult, 0, TotalAuctions)

	// Guard for write errors across goroutines.
	var writeErrMu sync.Mutex
	var writeErrors []error

	// File writes happen concurrently.
	var writeWg sync.WaitGroup

	for result := range resultCh {
		results = append(results, result)

		writeWg.Add(1)
		r := result // capture for goroutine
		go func() {
			defer writeWg.Done()
			path, err := WriteResult(r)
			if err != nil {
				writeErrMu.Lock()
				writeErrors = append(writeErrors, fmt.Errorf("auction %d: %w", r.AuctionID, err))
				writeErrMu.Unlock()
			} else {
				fmt.Printf("  ✓ Auction #%03d written → %s\n", r.AuctionID, path)
			}
		}()
	}

	// Wait for all file writes to complete before returning.
	writeWg.Wait()

	firstNano := firstStarted.Load()
	lastNano := lastEnded.Load()
	firstTime := time.Unix(0, firstNano)
	lastTime := time.Unix(0, lastNano)

	return SimulationSummary{
		TotalAuctions:  TotalAuctions,
		Results:        results,
		FirstStartedAt: firstTime,
		LastEndedAt:    lastTime,
		TotalDuration:  lastTime.Sub(firstTime),
		WriteErrors:    writeErrors,
	}
}

// Atomic min/max helpers.

// recordFirst stores v in a if the current value of a is 0 (unset) or > v.
func recordFirst(a *atomic.Int64, v int64) {
	for {
		old := a.Load()
		if old != 0 && old <= v {
			return
		}
		if a.CompareAndSwap(old, v) {
			return
		}
	}
}

// recordLast stores v in a if v > current value.
func recordLast(a *atomic.Int64, v int64) {
	for {
		old := a.Load()
		if old >= v {
			return
		}
		if a.CompareAndSwap(old, v) {
			return
		}
	}
}
