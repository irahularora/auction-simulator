// Package simulator handles the single auction lifecycle and its logic.
package simulator

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Attribute value pools - used to generate items.

var (
	attributeNames = []string{
		"Color", "Material", "Weight", "Condition", "Origin",
		"Age", "Rarity", "Size", "Shape", "Finish",
		"Texture", "Transparency", "Flexibility", "Conductivity", "Magnetism",
		"Reactivity", "Density", "Hardness", "Luminosity", "Solubility",
	}

	attributeValues = []string{
		"Excellent", "Good", "Fair", "Poor", "Unknown",
		"High", "Medium", "Low", "Very High", "Very Low",
		"Red", "Blue", "Green", "Black", "White",
		"Metal", "Plastic", "Wood", "Glass", "Ceramic",
	}
)

// RunAuction executes a single auction and returns its result.
func RunAuction(parentCtx context.Context, auctionID int) AuctionResult {
	// Create a fresh timeout independent of the parent's deadline.
	auctionCtx, cancelTimeout := context.WithTimeout(context.Background(), AuctionTimeout)
	defer cancelTimeout()

	// Still respect parent cancellation (Ctrl+C, SIGTERM).
	auctionCtx, cancelParent := context.WithCancelCause(auctionCtx)
	defer cancelParent(nil)
	go func() {
		select {
		case <-parentCtx.Done():
			cancelParent(parentCtx.Err())
		case <-auctionCtx.Done():
		}
	}()

	// Now derive startedAt from the deadline so the clock is exact.
	deadline, _ := auctionCtx.Deadline()
	startedAt := deadline.Add(-AuctionTimeout)

	// Deterministic RNG per auction.
	rng := rand.New(rand.NewSource(int64(auctionID) * 0xDEADBEEF)) //nolint:gosec

	item := generateItem(auctionID, rng)

	// Buffered channel for bids.
	bidCh := make(chan Bid, TotalBidders)

	var wg sync.WaitGroup
	wg.Add(TotalBidders)

	// Launch all bidder goroutines concurrently.
	for bidderID := 1; bidderID <= TotalBidders; bidderID++ {
		bID := bidderID
		bidderRng := rand.New(rand.NewSource(int64(auctionID)*1000 + int64(bID))) //nolint:gosec

		go func() {
			defer wg.Done()
			simulateBidder(auctionCtx, item, bID, bidderRng, bidCh)
		}()
	}

	// Close the bid channel after all bidder goroutines exit.
	go func() {
		wg.Wait()
		close(bidCh)
	}()

	// Collect all bids.
	allBids := make([]Bid, 0, TotalBidders)
	for bid := range bidCh {
		allBids = append(allBids, bid)
	}

	// An auction must strictly remain open for its entire duration limit.
	// Even if all 100 bidders happen to submit their bids internally early, 
	// the auction window mathematically stays open until the buzzer.
	<-auctionCtx.Done()

	closedAt := time.Now()
	var closedBy string

	if parentCtx.Err() != nil {
		closedBy = "cancelled"
	} else {
		closedBy = "timeout"
		closedAt = deadline
	}

	return AuctionResult{
		AuctionID: auctionID,
		Item:      item,
		AllBids:   allBids,
		Winner:    findWinner(allBids),
		StartedAt: startedAt,
		ClosedAt:  closedAt,
		Duration:  closedAt.Sub(startedAt),
		ClosedBy:  closedBy,
	}
}

// generateItem constructs an AuctionItem using the supplied RNG.
func generateItem(id int, rng *rand.Rand) AuctionItem {
	var item AuctionItem
	item.ID = id
	item.Attributes = make([]Attribute, AttributeCount)
	for i := range item.Attributes {
		nameIdx := i % len(attributeNames)
		item.Attributes[i] = Attribute{
			Name:  attributeNames[nameIdx],
			Value: attributeValues[rng.Intn(len(attributeValues))],
		}
	}
	return item
}

// simulateBidder simulates a single bidder reviewing the 20 attributes and taking time to decide on a bid.
func simulateBidder(ctx context.Context, item AuctionItem, bidderID int, rng *rand.Rand, bidCh chan<- Bid) {
	// Acknowledge receipt of the auction attributes: len(item.Attributes) == 20
	_ = item.Attributes 

	// Simulate variable decision delay.
	delay := time.Duration(rng.Int63n(int64(BidderMaxResponseDelay)))

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	// 30 % of bidders pass on this auction.
	if rng.Float64() < 0.30 {
		return
	}

	// Final context check before making a bid.
	select {
	case <-ctx.Done():
		return
	default:
	}

	amount := 100.0 + rng.Float64()*9900.0 // bid range: $100 – $10 000

	bid := Bid{
		AuctionID: item.ID,
		BidderID:  bidderID,
		Amount:    amount,
		PlacedAt:  time.Now(),
	}

	// Non-blocking send. Discard if channel is full.
	select {
	case bidCh <- bid:
	default:
		fmt.Printf("[warn] auction %d: bid channel full – bidder %d discarded\n",
			item.ID, bidderID)
	}
}

// findWinner returns a pointer to the highest-amount bid, or nil if none.
func findWinner(bids []Bid) *Bid {
	if len(bids) == 0 {
		return nil
	}
	winner := &bids[0]
	for i := 1; i < len(bids); i++ {
		if bids[i].Amount > winner.Amount {
			winner = &bids[i]
		}
	}
	return winner
}
