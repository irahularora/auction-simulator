// Package simulator handles writing auction output files.
package simulator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const resultsDir = "results"

// WriteResult writes auction details to an output file.
// Returns the path of the written file.
func WriteResult(result AuctionResult) (string, error) {
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		return "", fmt.Errorf("create results dir: %w", err)
	}

	fileName := fmt.Sprintf("auction_%03d.txt", result.AuctionID)
	filePath := filepath.Join(resultsDir, fileName)

	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create result file %s: %w", filePath, err)
	}
	defer f.Close()

	w := newLineWriter(f)
	writeHeader(w, result)
	writeItemAttributes(w, result)
	writeBidSummary(w, result)
	writeAllBids(w, result)
	writeWinner(w, result)
	writeFooter(w, result)

	return filePath, w.err
}

// Section writers

func writeHeader(w *lineWriter, r AuctionResult) {
	w.line(separator('=', 72))
	w.linef("  AUCTION RESULT – Auction #%03d", r.AuctionID)
	w.line(separator('=', 72))
	w.linef("  Started  : %s", r.StartedAt.Format("2006-01-02 15:04:05.000"))
	w.linef("  Closed   : %s", r.ClosedAt.Format("2006-01-02 15:04:05.000"))
	w.linef("  Duration : %dms", r.Duration.Milliseconds())
	closeReason := "Context cancelled"
	switch r.ClosedBy {
	case "completed":
		closeReason = "Completed (all bidders finished)"
	case "timeout":
		closeReason = "Timeout (natural expiry)"
	case "cancelled":
		closeReason = "Context cancelled"
	}
	w.linef("  Closed by: %s", closeReason)
	w.line("")
}

func writeItemAttributes(w *lineWriter, r AuctionResult) {
	w.line(separator('-', 72))
	w.line("  ITEM ATTRIBUTES")
	w.line(separator('-', 72))
	for _, attr := range r.Item.Attributes {
		w.linef("  %-18s : %s", attr.Name, attr.Value)
	}
	w.line("")
}

func writeBidSummary(w *lineWriter, r AuctionResult) {
	w.line(separator('-', 72))
	w.line("  BID SUMMARY")
	w.line(separator('-', 72))
	w.linef("  Total bidders  : %d", TotalBidders)
	w.linef("  Bids received  : %d", len(r.AllBids))
	w.linef("  Bids skipped   : %d", TotalBidders-len(r.AllBids))
	w.line("")
}

func writeAllBids(w *lineWriter, r AuctionResult) {
	w.line(separator('-', 72))
	w.line("  ALL BIDS (sorted by arrival)")
	w.line(separator('-', 72))
	if len(r.AllBids) == 0 {
		w.line("  (no bids received)")
	}
	for i, bid := range r.AllBids {
		marker := "  "
		if r.Winner != nil && bid.BidderID == r.Winner.BidderID {
			marker = "► " // highlight the winning bid
		}
		w.linef("%s[%02d] %s", marker, i+1, bid.String())
	}
	w.line("")
}

func writeWinner(w *lineWriter, r AuctionResult) {
	w.line(separator('-', 72))
	w.line("  WINNER")
	w.line(separator('-', 72))
	if r.Winner == nil {
		w.line("  No winner – auction received no bids.")
	} else {
		w.linef("  🏆 Bidder #%03d wins with $%.2f", r.Winner.BidderID, r.Winner.Amount)
	}
	w.line("")
}

func writeFooter(w *lineWriter, r AuctionResult) {
	w.line(separator('=', 72))
	w.linef("  END OF AUCTION #%03d", r.AuctionID)
	w.line(separator('=', 72))
}

// lineWriter accumulates write errors cleanly.

type lineWriter struct {
	f   *os.File
	err error
}

func newLineWriter(f *os.File) *lineWriter { return &lineWriter{f: f} }

func (w *lineWriter) line(s string) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintln(w.f, s)
}

func (w *lineWriter) linef(format string, args ...any) {
	if w.err != nil {
		return
	}
	_, w.err = fmt.Fprintf(w.f, format+"\n", args...)
}

// Helpers

func separator(ch rune, n int) string {
	return strings.Repeat(string(ch), n)
}
