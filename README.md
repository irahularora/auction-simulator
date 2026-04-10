# Auction Simulator

A production-grade, highly concurrent Auction Simulator written in idiomatic Go.

It runs **40 auctions** simultaneously, each with **100 bidders** responding to an item described by **20 attributes**, within a configurable timeout. The program accurately measures total execution span, respects hardware limits via a resource governor, and writes a dedicated result file per auction.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Project Structure](#project-structure)
3. [Architecture & Design Decisions](#architecture--design-decisions)
4. [Concurrency Model (detailed)](#concurrency-model-detailed)
5. [Resource Standardisation](#resource-standardisation)
6. [Configuration](#configuration)
7. [Output](#output)
8. [Evaluation Criteria Mapping](#evaluation-criteria-mapping)

---

## Quick Start

```bash
# Clone / enter the project
cd auction-simulator

# Build the executable (no external dependencies)
go build -o auction-simulator.exe .

# Run the simulation
.\auction-simulator.exe
```

Output files are written to `./results/`.

---

## Project Structure

```
auction-simulator/
├── main.go                   # Entry point: signal handling, timing, summary
├── go.mod                    # Module definition (no third-party deps)
├── implementation_plan.md    # Original task specification
├── results/                  # Created at runtime
│   ├── auction_001.txt       # Per-auction result (one file per auction)
│   ├── auction_002.txt
│   ├── …
│   └── summary.txt           # Aggregated summary of all auctions
└── simulator/
    ├── models.go             # Domain types: Attribute, AuctionItem, Bid, AuctionResult
    ├── auction.go            # Single auction lifecycle (bidder goroutines, timeout)
    ├── coordinator.go        # Runs all auctions concurrently, collects results
    ├── resource.go           # vCPU / RAM resource governor (semaphore)
    └── output.go             # Writes per-auction result files
```

---

## Architecture & Design Decisions

### Separation of concerns

Each file has **one job**:

| File | Responsibility |
|---|---|
| `models.go` | Pure data – no logic, easy to read and extend |
| `resource.go` | Hardware detection and concurrency cap – isolated from business logic |
| `auction.go` | One auction from start to finish – independently testable |
| `coordinator.go` | Fan-out / fan-in orchestration – does not know about file I/O |
| `output.go` | File I/O – no concurrency primitives beyond what `os.Create` provides |
| `main.go` | Wires everything together, handles signals and the final report |

### No external dependencies

The entire program depends only on the Go standard library. This makes the binary fully portable and trivially reproducible across machines.

---

## Concurrency Model (detailed)

```
main()
 └─ simulator.Run(ctx, governor)
      │
      ├─ for i in 1..40:
      │     governor.Acquire()           ← blocks if at hardware capacity
      │     go RunAuction(ctx, i)        ← goroutine: one full auction
      │         │
      │         ├─ context.WithTimeout(AuctionTimeout)
      │         ├─ for j in 1..100:
      │         │     go simulateBidder(auctionCtx, j, bidCh)
      │         │         ├─ time.After(randomDelay)  ← simulates network latency
      │         │         ├─ 30% chance: pass (no bid)
      │         │         └─ bidCh <- Bid{}            ← non-blocking send
      │         │
      │         ├─ go wg.Wait() → close(bidCh)         ← safe channel close
      │         └─ for bid := range bidCh              ← drains until closed
      │               allBids = append(allBids, bid)
      │
      └─ resultCh <- AuctionResult{}
           │
           └─ collector: for result := range resultCh
                 go WriteResult(result)   ← concurrent file writes
```

### Key patterns used

| Pattern | Where | Why |
|---|---|---|
| `context.WithTimeout` | `auction.go` | Natural timeout propagation without manual timers |
| `context.WithCancel` (via `signal.NotifyContext`) | `main.go` | OS signal → clean shutdown of all goroutines |
| Buffered channel as semaphore | `resource.go` | Limits concurrency to hardware capacity – O(1) acquire/release |
| Buffered result/bid channels | `coordinator.go`, `auction.go` | Senders never block on receivers |
| `sync.WaitGroup` | `auction.go`, `coordinator.go` | Safe channel close – prevents send-on-closed panics |
| `sync/atomic` CAS loop | `coordinator.go` | Lock-free min/max for timing sentinels |
| `sync.Mutex` | `coordinator.go` | Guards the write-error slice (low-contention) |

### Why not a `select` on a "done" channel to close `bidCh`?

A common mistake is to close a channel from a sender when the timeout fires. If two bidder goroutines reach the send simultaneously after the close, the program panics. The correct pattern (used here) is:

1. Sender goroutines check `ctx.Done()` and **return** instead of sending.  
2. A separate goroutine calls `wg.Wait()` then `close(bidCh)` – guaranteed to run only after every sender has exited.

---

## Resource Standardisation

`simulator/resource.go` implements a `ResourceGovernor` that:

1. **Reads logical CPU count** via `runtime.NumCPU()` and sets `GOMAXPROCS` to match, ensuring the Go scheduler uses all available cores.
2. **Reads current heap statistics** (`runtime.ReadMemStats`) to estimate free RAM.
3. **Calculates a concurrency cap**:

   ```
   cpuCap  = logicalCPUs × 4          (I/O-heavy goroutines can over-subscribe)
   ramCap  = freeHeap / 8 KB           (8 KB per auction is a conservative bound)
   maxConc = min(cpuCap, ramCap, 40)   (never exceed TotalAuctions)
   ```

4. **Exposes a semaphore** (`Acquire` / `Release`) backed by a buffered channel of capacity `maxConc`. The coordinator calls `Acquire()` before spawning each auction goroutine, so at most `maxConc` auctions are alive at any one moment.

This means the simulator **self-tunes** to the machine it runs on: a 2-core laptop will run fewer auctions at once than a 32-core server, but the final results are identical.

---

## Configuration

All tunable constants live in `simulator/models.go` and `simulator/resource.go`:

| Constant | Default | Description |
|---|---|---|
| `TotalAuctions` | `40` | Number of concurrent auctions |
| `TotalBidders` | `100` | Bidding agents per auction |
| `AttributeCount` | `20` | Descriptors on each item |
| `AuctionTimeout` | `500ms` | How long each auction stays open |
| `BidderMaxResponseDelay` | `600ms` | Max random delay before a bidder decides |
| `cpuMultiplier` | `4` | Goroutines per logical CPU |
| `peakMemoryPerAuction` | `8 KB` | RAM estimate per running auction |

---

## Output

### Console (stdout)

```
╔══════════════════════════════════════════════════════════════════════╗
║               AUCTION SIMULATOR  –  Production Grade                ║
╚══════════════════════════════════════════════════════════════════════╝

Resource Governor │ logical CPUs: 8 │ GOMAXPROCS: 8 │ heap (sys): 3.8 MB │ max concurrency: 32

Starting 40 concurrent auctions with 100 bidders each…

  ✓ Auction #007 written → results/auction_007.txt
  ✓ Auction #012 written → results/auction_012.txt
  …

─────────────────────────────────────────────────────────────────────────
  SIMULATION COMPLETE

  Total auctions run  : 40
  First auction start : 14:05:33.021
  Last  auction end   : 14:05:33.541
  Auction span        : 520ms   (first-start → last-end)
  Wall-clock total    : 534ms   (includes startup & file I/O)

  ID     Bids  Winner       WinAmount   Duration  ClosedBy
  ──     ────  ──────       ─────────   ────────  ────────
  #001   71    Bidder #042  $9847.23    501ms     timeout
  #002   68    Bidder #017  $9923.55    500ms     timeout
  …
```

### Per-auction file (`results/auction_NNN.txt`)

```
════════════════════════════════════════════════════════════════════════
  AUCTION RESULT – Auction #001
════════════════════════════════════════════════════════════════════════
  Started  : 2026-04-09 14:05:33.021
  Closed   : 2026-04-09 14:05:33.522
  Duration : 501ms
  Closed by: Timeout (natural expiry)

────────────────────────────────────────────────────────────────────────
  ITEM ATTRIBUTES
────────────────────────────────────────────────────────────────────────
  Color              : Red
  Material           : Metal
  …

────────────────────────────────────────────────────────────────────────
  ALL BIDS (sorted by arrival)
────────────────────────────────────────────────────────────────────────
  [01] Bidder #003 → $4521.90 (at 14:05:33.145)
► [02] Bidder #042 → $9847.23 (at 14:05:33.287)   ← winner marked with ►
  …

  🏆 Bidder #042 wins with $9847.23
```

### Summary file (`results/summary.txt`)

All-auction table in the same format as the console output, plus configuration metadata and timing.

---

## Evaluation Criteria Mapping

| Criterion | Implementation |
|---|---|
| **Correctness** | Each auction opens, collects bids concurrently, closes on timeout, and declares the highest bidder as winner. No-bid auctions produce "no winner". |
| **Time Measurement** | `firstStarted` / `lastEnded` captured atomically inside goroutines; reported as "Auction span" in summary. |
| **Resource Standardisation** | `ResourceGovernor` reads CPU count and free RAM at startup, sets `GOMAXPROCS`, and caps concurrency via a semaphore. |
| **Clarity** | Every file has a package-level doc comment explaining its purpose and design decisions. All non-trivial logic is commented inline. |
