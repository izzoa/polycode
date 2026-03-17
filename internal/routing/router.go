package routing

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/izzoa/polycode/internal/provider"
	"github.com/izzoa/polycode/internal/telemetry"
)

// staleDuration is the maximum age of cached stats before they are refreshed
// from disk.
const staleDuration = 5 * time.Minute

// ProviderStats holds aggregated telemetry statistics for a single provider.
type ProviderStats struct {
	ProviderID      string
	AvgLatencyMS    float64
	ErrorRate       float64
	TotalSuccessful int
}

// Router selects providers based on telemetry-derived heuristic scores.
type Router struct {
	stats         map[string]ProviderStats
	statsTime     time.Time
	telemetryPath string
	mu            sync.RWMutex
}

// NewRouter creates a Router that reads telemetry data from the given path.
func NewRouter(telemetryPath string) *Router {
	return &Router{
		stats:         make(map[string]ProviderStats),
		telemetryPath: telemetryPath,
	}
}

// LoadTelemetryStats reads the telemetry JSONL file and aggregates per-provider
// statistics (average latency, error rate, total successful queries).
func (r *Router) LoadTelemetryStats() error {
	f, err := os.Open(r.telemetryPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.mu.Lock()
			r.stats = make(map[string]ProviderStats)
			r.statsTime = time.Now()
			r.mu.Unlock()
			return nil
		}
		return err
	}
	defer f.Close()

	type accumulator struct {
		totalLatency float64
		totalQueries int
		successCount int
		errorCount   int
	}

	accum := make(map[string]*accumulator)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev telemetry.Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}

		if ev.EventType != telemetry.EventProviderResponse {
			continue
		}
		if ev.ProviderID == "" {
			continue
		}

		a, ok := accum[ev.ProviderID]
		if !ok {
			a = &accumulator{}
			accum[ev.ProviderID] = a
		}

		a.totalQueries++
		if ev.Success != nil && *ev.Success {
			a.successCount++
			a.totalLatency += float64(ev.LatencyMS)
		} else {
			a.errorCount++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	stats := make(map[string]ProviderStats, len(accum))
	for pid, a := range accum {
		var avgLatency float64
		if a.successCount > 0 {
			avgLatency = a.totalLatency / float64(a.successCount)
		}
		var errorRate float64
		if a.totalQueries > 0 {
			errorRate = float64(a.errorCount) / float64(a.totalQueries)
		}
		stats[pid] = ProviderStats{
			ProviderID:      pid,
			AvgLatencyMS:    avgLatency,
			ErrorRate:       errorRate,
			TotalSuccessful: a.successCount,
		}
	}

	r.mu.Lock()
	r.stats = stats
	r.statsTime = time.Now()
	r.mu.Unlock()

	return nil
}

// ScoreProvider computes a heuristic score for a provider based on its stats.
// Higher scores indicate better providers. Providers with zero history receive
// a neutral score of 1.0.
func (r *Router) ScoreProvider(stats ProviderStats) float64 {
	if stats.TotalSuccessful == 0 && stats.ErrorRate == 0 {
		return 1.0 // neutral score for providers with no history
	}
	if stats.AvgLatencyMS <= 0 {
		return 0.0
	}
	return (1.0 / stats.AvgLatencyMS) * (1.0 - stats.ErrorRate) * math.Log(float64(stats.TotalSuccessful)+1)
}

// refreshIfStale reloads stats from disk if they are older than staleDuration.
func (r *Router) refreshIfStale() {
	r.mu.RLock()
	stale := time.Since(r.statsTime) > staleDuration
	r.mu.RUnlock()

	if stale {
		_ = r.LoadTelemetryStats()
	}
}

// SelectProviders returns a subset of allHealthy providers based on the given
// mode and heuristic scores.
//
//   - quick:    returns only the primary provider
//   - balanced: returns the primary plus the best-scoring non-primary
//   - thorough: returns all healthy providers
func (r *Router) SelectProviders(mode Mode, allHealthy []provider.Provider, primaryID string) []provider.Provider {
	r.refreshIfStale()

	switch mode {
	case ModeQuick:
		for _, p := range allHealthy {
			if p.ID() == primaryID {
				return []provider.Provider{p}
			}
		}
		// Primary not found in healthy list; return first available.
		if len(allHealthy) > 0 {
			return []provider.Provider{allHealthy[0]}
		}
		return nil

	case ModeBalanced:
		var primary provider.Provider
		var secondaries []provider.Provider

		for _, p := range allHealthy {
			if p.ID() == primaryID {
				primary = p
			} else {
				secondaries = append(secondaries, p)
			}
		}

		result := make([]provider.Provider, 0, 2)
		if primary != nil {
			result = append(result, primary)
		}

		if len(secondaries) > 0 {
			r.mu.RLock()
			stats := r.stats
			r.mu.RUnlock()

			sort.Slice(secondaries, func(i, j int) bool {
				si := stats[secondaries[i].ID()]
				sj := stats[secondaries[j].ID()]
				return r.ScoreProvider(si) > r.ScoreProvider(sj)
			})
			result = append(result, secondaries[0])
		}

		return result

	case ModeThorough:
		return allHealthy

	default:
		// Fallback to balanced behavior.
		return r.SelectProviders(ModeBalanced, allHealthy, primaryID)
	}
}
