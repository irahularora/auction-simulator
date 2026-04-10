// Package simulator contains core domain logic for the auction simulator.
package simulator

import (
	"fmt"
	"strings"
	"time"
)

// Configuration Constants

var (
	TotalAuctions          = 40
	TotalBidders           = 100
	AttributeCount         = 20
	AuctionTimeout         = 500 * time.Millisecond
	BidderMaxResponseDelay = 600 * time.Millisecond
)

// Attribute represents a descriptor of the auctioned object.
type Attribute struct {
	Name  string // Human-readable descriptor name.
	Value string // The value of the descriptor.
}

// String returns a formatted key=value representation.
func (a Attribute) String() string {
	return fmt.Sprintf("%s=%s", a.Name, a.Value)
}

// AuctionItem is the object being sold.
type AuctionItem struct {
	ID         int
	Attributes []Attribute
}

// AttributeSummary returns a compact single-line description of the item.
func (item *AuctionItem) AttributeSummary() string {
	parts := make([]string, AttributeCount)
	for i, attr := range item.Attributes {
		parts[i] = attr.String()
	}
	return strings.Join(parts, ", ")
}

// Bid holds the offer submitted by a single bidder.
type Bid struct {
	AuctionID int
	BidderID  int
	Amount    float64
	PlacedAt  time.Time // Wall-clock time when the bid was placed.
}

// String formats the bid for display in output files and console logs.
func (b Bid) String() string {
	return fmt.Sprintf("Bidder #%03d → $%.2f (at %s)",
		b.BidderID, b.Amount, b.PlacedAt.Format("15:04:05.000"))
}

// AuctionResult captures everything that happened during one auction.
type AuctionResult struct {
	AuctionID int
	Item      AuctionItem
	AllBids   []Bid     // Every bid received before the timeout.
	Winner    *Bid      // nil when no bids were received.
	StartedAt time.Time // When the auction goroutine began.
	ClosedAt  time.Time // When the auction closed (timeout or context cancel).
	Duration  time.Duration
	ClosedBy  string // "timeout", "cancelled", or "completed"
}
