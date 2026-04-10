// Package simulator handles resource limitation using a semaphore pattern.
package simulator

import (
	"fmt"
	"runtime"
)

// Internal tuning constants

const (
	// cpuMultiplier is the number of goroutines to run per physical core.
	cpuMultiplier = 4

	// peakMemoryPerAuction is a conservative heap estimate per running auction.
	peakMemoryPerAuction = 8 * 1024 // bytes
)

// ResourceGovernor manages maximum concurrency for auctions.
type ResourceGovernor struct {
	cfg       Config
	semaphore chan struct{} // buffered channel acting as counting semaphore
}

// NewResourceGovernor initializes a resource limit governor.
func NewResourceGovernor(cfg Config) *ResourceGovernor {
	// Apply the CPU limit to the Go scheduler.
	runtime.GOMAXPROCS(cfg.CPULimit)

	return &ResourceGovernor{
		cfg:       cfg,
		semaphore: make(chan struct{}, cfg.MaxWorkers),
	}
}

// Acquire blocks until a concurrency slot becomes available.
func (g *ResourceGovernor) Acquire() {
	g.semaphore <- struct{}{}
}

// Release frees a concurrency slot.
func (g *ResourceGovernor) Release() {
	<-g.semaphore
}

// MaxConcurrency returns the resolved maximum number of simultaneous goroutines.
func (g *ResourceGovernor) MaxConcurrency() int {
	return g.cfg.MaxWorkers
}

// RuntimeSummary returns a snapshot of memory and goroutine stats.
func (g *ResourceGovernor) RuntimeSummary() string {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return fmt.Sprintf(
		"Runtime snapshot │ GOMAXPROCS: %d │ HeapAlloc: %s │ HeapSys: %s │ NumGoroutine: %d",
		runtime.GOMAXPROCS(0),
		formatBytes(mem.HeapAlloc),
		formatBytes(mem.HeapSys),
		runtime.NumGoroutine(),
	)
}

// Helpers

// min3 returns the smallest of three integers.
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// formatBytes converts a byte count to a human-readable string (KB, MB, …).
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
