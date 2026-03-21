package routing

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
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
	AcceptRate      float64 // user feedback: fraction of accepted tool calls
	FeedbackCount   int     // total feedback events
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
		totalLatency  float64
		totalQueries  int
		successCount  int
		errorCount    int
		acceptCount   int
		rejectCount   int
		feedbackCount int
	}

	accum := make(map[string]*accumulator)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev telemetry.Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // skip malformed lines
		}

		if ev.ProviderID == "" {
			continue
		}

		a, ok := accum[ev.ProviderID]
		if !ok {
			a = &accumulator{}
			accum[ev.ProviderID] = a
		}

		switch ev.EventType {
		case telemetry.EventProviderResponse:
			a.totalQueries++
			if ev.Success != nil && *ev.Success {
				a.successCount++
				a.totalLatency += float64(ev.LatencyMS)
			} else {
				a.errorCount++
			}
		case telemetry.EventUserFeedback:
			a.feedbackCount++
			if ev.Accepted != nil && *ev.Accepted {
				a.acceptCount++
			} else {
				a.rejectCount++
			}
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
		var acceptRate float64
		if a.feedbackCount > 0 {
			acceptRate = float64(a.acceptCount) / float64(a.feedbackCount)
		} else {
			acceptRate = 1.0 // neutral if no feedback
		}
		stats[pid] = ProviderStats{
			ProviderID:      pid,
			AvgLatencyMS:    avgLatency,
			ErrorRate:       errorRate,
			TotalSuccessful: a.successCount,
			AcceptRate:      acceptRate,
			FeedbackCount:   a.feedbackCount,
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
// a neutral score of 1.0. The score factors in latency, error rate, volume,
// and user acceptance rate from feedback signals.
func (r *Router) ScoreProvider(stats ProviderStats) float64 {
	if stats.TotalSuccessful == 0 && stats.ErrorRate == 0 {
		return 1.0 // neutral score for providers with no history
	}
	if stats.AvgLatencyMS <= 0 {
		return 0.0
	}
	base := (1.0 / stats.AvgLatencyMS) * (1.0 - stats.ErrorRate) * math.Log(float64(stats.TotalSuccessful)+1)

	// Weight by user feedback acceptance rate if we have enough data.
	// AcceptRate defaults to 1.0 if no feedback, so this is a no-op when
	// there's no user signal.
	acceptWeight := stats.AcceptRate
	if stats.FeedbackCount == 0 {
		acceptWeight = 1.0
	}
	return base * acceptWeight
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

// SelectProviders returns all healthy providers regardless of mode.
// The mode now controls synthesis depth, not provider selection.
func (r *Router) SelectProviders(mode Mode, allHealthy []provider.Provider, primaryID string) []provider.Provider {
	providers, _ := r.SelectProvidersWithReason(mode, allHealthy, primaryID)
	return providers
}

// SelectProvidersWithReason returns all healthy providers along with a
// human-readable explanation. All modes query all providers — the mode
// controls synthesis depth (quick=concise, balanced=structured, thorough=deep).
func (r *Router) SelectProvidersWithReason(mode Mode, allHealthy []provider.Provider, primaryID string) ([]provider.Provider, string) {
	r.refreshIfStale()
	reason := fmt.Sprintf("%s: all %d providers, %s synthesis", mode, len(allHealthy), mode)
	return allHealthy, reason
}
