package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auction-simulator/simulator"
)

func main() {
	// Setup root context with OS signal handling for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	simulator.PrintBanner()

	// Load configuration from environment or auto-detect from hardware.
	cfg := simulator.LoadConfig()
	fmt.Println(cfg.Summary())

	// Build resource governor to enforce worker concurrency limits.
	governor := simulator.NewResourceGovernor(cfg)

	fmt.Printf("Starting %d concurrent auctions · %d bidders each · timeout %dms\n\n",
		simulator.TotalAuctions, simulator.TotalBidders, simulator.AuctionTimeout.Milliseconds())

	wallStart := time.Now()

	// Run all auctions concurrently.
	summary := simulator.Run(ctx, governor)
	wallElapsed := time.Since(wallStart)

	// Print summary and write report files.
	simulator.PrintSummary(summary, wallElapsed, governor)

	if err := simulator.WriteSummaryFile(summary, wallElapsed, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "\n[warn] could not write summary file: %v\n", err)
	}

	// Non-fatal file-write errors collected during the run.
	if len(summary.WriteErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d result file(s) had write errors:\n", len(summary.WriteErrors))
		for _, e := range summary.WriteErrors {
			fmt.Fprintf(os.Stderr, "  • %v\n", e)
		}
		os.Exit(1)
	}
}
