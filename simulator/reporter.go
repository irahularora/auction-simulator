// Package simulator handles reporting logic.
package simulator

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"
)

// PrintBanner prints the title banner.
func PrintBanner() {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════════════╗
║               AUCTION SIMULATOR  –  Production Grade                ║
║        40 Concurrent Auctions  ×  100 Bidders  ×  20 Attributes     ║
╚══════════════════════════════════════════════════════════════════════╝`)
}

func PrintSummary(s SimulationSummary, wallElapsed time.Duration, gov *ResourceGovernor) {
	sep := "─────────────────────────────────────────────────────────────────────────"

	fmt.Printf("\n%s\n  SIMULATION COMPLETE\n%s\n\n", sep, sep)

	fmt.Printf("  Total auctions run  : %d\n", s.TotalAuctions)
	fmt.Printf("  First auction start : %s\n", s.FirstStartedAt.Format("15:04:05.000"))
	fmt.Printf("  Last  auction end   : %s\n", s.LastEndedAt.Format("15:04:05.000"))
	fmt.Printf("  Auction span        : %dms  (first-start → last-end)\n",
		s.TotalDuration.Milliseconds())
	fmt.Printf("  Wall-clock total    : %dms  (includes startup & file I/O)\n",
		wallElapsed.Milliseconds())
	fmt.Printf("\n  %s\n", gov.RuntimeSummary())

	fmt.Printf("\n%s\n  PER-AUCTION RESULTS\n%s\n\n", sep, sep)

	sorted := sortedResults(s.Results)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  ID\tBids\tWinner\tWinAmount\tDuration\tClosedBy")
	fmt.Fprintln(tw, "  ──\t────\t──────\t─────────\t────────\t────────")
	for _, r := range sorted {
		winner, amount := winnerFields(r)
		fmt.Fprintf(tw, "  #%03d\t%d\t%s\t%s\t%dms\t%s\n",
			r.AuctionID, len(r.AllBids), winner, amount,
			r.Duration.Milliseconds(), r.ClosedBy)
	}
	_ = tw.Flush()

	fmt.Printf("\n%s\n", sep)
	fmt.Printf("  Per-auction files → results/auction_NNN.txt\n")
	fmt.Printf("  Full summary      → results/summary.txt\n")
	fmt.Printf("%s\n\n", sep)
}

func WriteSummaryFile(s SimulationSummary, wallElapsed time.Duration, cfg Config) error {
	if err := os.MkdirAll("results", 0o755); err != nil {
		return fmt.Errorf("create results dir: %w", err)
	}

	f, err := os.Create("results/summary.txt")
	if err != nil {
		return fmt.Errorf("open summary.txt: %w", err)
	}
	defer f.Close()

	// Header
	fmt.Fprintln(f, "AUCTION SIMULATOR – SUMMARY REPORT")
	fmt.Fprintln(f, "Generated:", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(f)

	// Configuration block
	fmt.Fprintln(f, cfg.Summary())

	// Simulation parameters
	fmt.Fprintln(f, "Simulation Parameters")
	fmt.Fprintf(f, "  Total auctions : %d\n", TotalAuctions)
	fmt.Fprintf(f, "  Total bidders  : %d\n", TotalBidders)
	fmt.Fprintf(f, "  Attributes     : %d\n", AttributeCount)
	fmt.Fprintln(f)

	// Timing
	fmt.Fprintln(f, "Timing")
	fmt.Fprintf(f, "  First auction start : %s\n", s.FirstStartedAt.Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(f, "  Last  auction end   : %s\n", s.LastEndedAt.Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(f, "  Auction span        : %dms\n", s.TotalDuration.Milliseconds())
	fmt.Fprintf(f, "  Wall-clock total    : %dms\n", wallElapsed.Milliseconds())
	fmt.Fprintln(f)

	// Per-auction table
	fmt.Fprintln(f, "Results")
	tw := tabwriter.NewWriter(f, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tBids\tWinner\tWinAmount\tDuration\tClosedBy")
	for _, r := range sortedResults(s.Results) {
		winner, amount := winnerFields(r)
		fmt.Fprintf(tw, "#%03d\t%d\t%s\t%s\t%dms\t%s\n",
			r.AuctionID, len(r.AllBids), winner, amount,
			r.Duration.Milliseconds(), r.ClosedBy)
	}
	return tw.Flush()
}

// sortedResults returns a copy of results sorted ascending by AuctionID.
func sortedResults(results []AuctionResult) []AuctionResult {
	sorted := make([]AuctionResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].AuctionID < sorted[j].AuctionID
	})
	return sorted
}

// winnerFields returns display strings for the winner columns.
func winnerFields(r AuctionResult) (winner, amount string) {
	if r.Winner == nil {
		return "none", "-"
	}
	return fmt.Sprintf("Bidder #%03d", r.Winner.BidderID),
		fmt.Sprintf("$%.2f", r.Winner.Amount)
}

// removed closedByLabel
