// Package simulator handles configuration logic.
package simulator

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config holds all configurable parameters for the simulation.
type Config struct {
	// CPULimit is the number of logical CPUs the Go runtime may use.
	CPULimit int

	// RAMLimitMB is the RAM budget in megabytes.
	RAMLimitMB int

	// MaxWorkers is the maximum number of concurrent auctions.
	MaxWorkers int

	sources map[string]string // field name -> source
}

// LoadConfig detects hardware values and applies environment overrides.
func LoadConfig() Config {
	loadDotEnv() // Load .env file before reading environment settings

	logicalCPUs := runtime.NumCPU()

	// Read available memory correctly.
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	freeHeapMB := int((mem.HeapIdle + mem.HeapReleased) / (1024 * 1024))
	if freeHeapMB < 16 {
		freeHeapMB = 16 // safety floor: always allow at least 16 MB
	}

	cfg := Config{
		CPULimit:   logicalCPUs,
		RAMLimitMB: freeHeapMB,
		sources:    make(map[string]string),
	}
	cfg.sources["CPULimit"] = "auto"
	cfg.sources["RAMLimitMB"] = "auto"

	// Apply environment overrides.

	if v, ok := envInt("AUCTION_CPU_LIMIT"); ok {
		cfg.CPULimit = clamp(v, 1, logicalCPUs*2)
		cfg.sources["CPULimit"] = "AUCTION_CPU_LIMIT"
	}

	if v, ok := envInt("AUCTION_RAM_LIMIT_MB"); ok {
		cfg.RAMLimitMB = clamp(v, 1, 1024*1024) // max 1 TB is silly but safe
		cfg.sources["RAMLimitMB"] = "AUCTION_RAM_LIMIT_MB"
	}



	// Derive MaxWorkers based on limits.
	cpuCap := cfg.CPULimit * cpuMultiplier

	// Ram cap based on peak memory.
	ramCap := (cfg.RAMLimitMB * 1024 * 1024) / peakMemoryPerAuction
	if ramCap < 1 {
		ramCap = 1
	}

	cfg.MaxWorkers = min3(cpuCap, ramCap, TotalAuctions)
	cfg.sources["MaxWorkers"] = fmt.Sprintf(
		"derived: min(cpuCap=%d, ramCap=%d, auctions=%d)",
		cpuCap, ramCap, TotalAuctions,
	)

	// Allow an explicit override after derivation.
	if v, ok := envInt("AUCTION_MAX_WORKERS"); ok {
		cfg.MaxWorkers = v
		cfg.sources["MaxWorkers"] = "AUCTION_MAX_WORKERS"
	}

	if v, ok := envInt("AUCTION_TIMEOUT_MS"); ok {
		AuctionTimeout = time.Duration(v) * time.Millisecond
	}

	if v, ok := envInt("AUCTION_TOTAL_AUCTIONS"); ok {
		TotalAuctions = v
	}

	if v, ok := envInt("AUCTION_TOTAL_BIDDERS"); ok {
		TotalBidders = v
	}

	if v, ok := envInt("AUCTION_ATTRIBUTE_COUNT"); ok {
		AttributeCount = v
	}

	if v, ok := envInt("AUCTION_BIDDER_MAX_RESPONSE_DELAY_MS"); ok {
		BidderMaxResponseDelay = time.Duration(v) * time.Millisecond
	}

	return cfg
}

// Summary returns a formatted block documenting active resource settings.
func (c Config) Summary() string {
	var sb strings.Builder

	line := func(label, value, source string) {
		fmt.Fprintf(&sb, "  %-22s : %-12s  [%s]\n", label, value, source)
	}

	sb.WriteString("Resource Configuration\n")
	sb.WriteString("  " + strings.Repeat("─", 65) + "\n")
	line("vCPU limit", fmt.Sprintf("%d cores", c.CPULimit), c.sources["CPULimit"])
	line("RAM limit", fmt.Sprintf("%d MB", c.RAMLimitMB), c.sources["RAMLimitMB"])
	line("Max concurrent workers", fmt.Sprintf("%d", c.MaxWorkers), c.sources["MaxWorkers"])
	line("Auction timeout", fmt.Sprintf("%dms", AuctionTimeout.Milliseconds()), "dynamic")
	sb.WriteString("\n")
	sb.WriteString("  Override via environment variables:\n")
	sb.WriteString("    AUCTION_CPU_LIMIT=<n>      number of vCPUs to use\n")
	sb.WriteString("    AUCTION_RAM_LIMIT_MB=<n>   RAM budget in megabytes\n")
	sb.WriteString("    AUCTION_MAX_WORKERS=<n>    hard cap on concurrent auctions\n")
	sb.WriteString("    AUCTION_TIMEOUT_MS=<n>     per-auction timeout in milliseconds\n")

	return sb.String()
}

// envInt reads an environment variable as a positive integer.
func envInt(key string) (int, bool) {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil || v <= 0 {
		fmt.Fprintf(os.Stderr, "[warn] %s=%q is not a positive integer – ignoring\n", key, s)
		return 0, false
	}
	return v, true
}

// loadDotEnv reads the .env file in the current directory and sets standard environment variables.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return // Silently ignore if file doesn't exist
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`) // safely remove simple quotes
			// Only set if not already set in the environment
			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
	}
}

// clamp returns v bounded to [lo, hi].
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
