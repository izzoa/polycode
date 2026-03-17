package tokens

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

// DefaultMetadataURL is the default URL for litellm model metadata.
const DefaultMetadataURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// ModelInfo holds parsed metadata for a single model from the litellm database.
type ModelInfo struct {
	MaxInputTokens          int  `json:"max_input_tokens"`
	MaxOutputTokens         int  `json:"max_output_tokens"`
	SupportsFunctionCalling bool `json:"supports_function_calling"`
	SupportsVision          bool `json:"supports_vision"`
	SupportsReasoning       bool `json:"supports_reasoning"`
	SupportsResponseSchema  bool `json:"supports_response_schema"`
}

// FetchMetadata fetches the litellm model metadata JSON from the given URL.
// It returns the raw JSON bytes on success.
func FetchMetadata(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching metadata: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading metadata response: %w", err)
	}
	return data, nil
}

// ParseMetadata parses raw litellm JSON into a map of model name → ModelInfo.
// Unknown fields in each entry are silently ignored.
func ParseMetadata(data []byte) (map[string]ModelInfo, error) {
	// The litellm JSON is a map[string]object where each value has known fields
	// plus many others we don't care about. We use json.RawMessage first, then
	// unmarshal each entry individually to tolerate unknown fields.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing metadata JSON: %w", err)
	}

	result := make(map[string]ModelInfo, len(raw))
	for key, entry := range raw {
		var info ModelInfo
		if err := json.Unmarshal(entry, &info); err != nil {
			// Skip entries that can't be parsed (e.g., non-object values)
			continue
		}
		result[key] = info
	}
	return result, nil
}

// LoadCachedMetadata reads cached metadata JSON from disk.
// Returns the raw bytes, the file modification time, and any error.
func LoadCachedMetadata(path string) ([]byte, time.Time, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stat cache file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("reading cache file: %w", err)
	}

	return data, stat.ModTime(), nil
}

// SaveCachedMetadata writes raw JSON to the cache path with 0600 permissions.
// Parent directories are created if needed.
func SaveCachedMetadata(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}
	return nil
}

// MetadataStore wraps parsed litellm model metadata and provides lookup methods.
type MetadataStore struct {
	models map[string]ModelInfo
}

// NewMetadataStore creates a MetadataStore by orchestrating the fetch/cache/fallback flow:
//  1. Check cache mtime vs TTL — if fresh, use cache
//  2. If stale or missing, fetch from URL
//  3. On fetch success, parse and save cache
//  4. On fetch failure, fall back to stale cache
//  5. If no cache available, fall back to an empty map
//
// Errors are logged as warnings but never returned — the store always succeeds.
func NewMetadataStore(url string, cachePath string, cacheTTL time.Duration) (*MetadataStore, error) {
	if url == "" {
		url = DefaultMetadataURL
	}

	store := &MetadataStore{
		models: make(map[string]ModelInfo),
	}

	// Try loading cached data
	cachedData, mtime, cacheErr := LoadCachedMetadata(cachePath)
	cacheIsFresh := cacheErr == nil && time.Since(mtime) < cacheTTL

	if cacheIsFresh {
		// Cache is fresh — parse and use it
		models, err := ParseMetadata(cachedData)
		if err != nil {
			log.Printf("Warning: failed to parse cached metadata: %v", err)
		} else {
			store.models = models
			return store, nil
		}
	}

	// Cache is stale or missing — try fetching
	fetchedData, fetchErr := FetchMetadata(url, 5*time.Second)
	if fetchErr == nil {
		models, parseErr := ParseMetadata(fetchedData)
		if parseErr == nil {
			store.models = models
			// Save to cache
			if err := SaveCachedMetadata(cachePath, fetchedData); err != nil {
				log.Printf("Warning: failed to save metadata cache: %v", err)
			}
			return store, nil
		}
		log.Printf("Warning: failed to parse fetched metadata: %v", parseErr)
	} else {
		log.Printf("Warning: failed to fetch model metadata: %v", fetchErr)
	}

	// Fetch failed — fall back to stale cache if available
	if cacheErr == nil && cachedData != nil {
		models, err := ParseMetadata(cachedData)
		if err != nil {
			log.Printf("Warning: failed to parse stale cached metadata: %v", err)
		} else {
			store.models = models
			log.Printf("Warning: using stale metadata cache (fetch failed)")
			return store, nil
		}
	}

	// No cache, no fetch — fall back to empty map
	log.Printf("Warning: no model metadata available, using hardcoded limits only")
	return store, nil
}

// Lookup searches for a model in the metadata store using multi-strategy matching:
//  1. Exact match on the key
//  2. Try "{providerType}/{model}"
//  3. Scan keys ending with "/{model}"
func (s *MetadataStore) Lookup(model string, providerType string) (ModelInfo, bool) {
	// 1. Exact match
	if info, ok := s.models[model]; ok {
		return info, true
	}

	// 2. Try provider-prefixed key
	if providerType != "" {
		prefixed := providerType + "/" + model
		if info, ok := s.models[prefixed]; ok {
			return info, true
		}
	}

	// 3. Scan for keys ending with /{model}
	suffix := "/" + model
	for key, info := range s.models {
		if strings.HasSuffix(key, suffix) {
			return info, true
		}
	}

	return ModelInfo{}, false
}

// LimitForModel returns the context window limit for a model using three-tier fallback:
//  1. Config override (if > 0)
//  2. Litellm max_input_tokens from metadata
//  3. Hardcoded KnownLimits
//  4. 0 (unlimited)
func (s *MetadataStore) LimitForModel(model string, providerType string, configOverride int) int {
	if configOverride > 0 {
		return configOverride
	}

	if info, ok := s.Lookup(model, providerType); ok && info.MaxInputTokens > 0 {
		return info.MaxInputTokens
	}

	if limit, ok := KnownLimits[model]; ok {
		return limit
	}

	return 0
}

// CapabilitiesForModel returns the parsed capabilities for a model.
// If the model is not found, a zero-value ModelInfo is returned.
func (s *MetadataStore) CapabilitiesForModel(model string, providerType string) ModelInfo {
	if info, ok := s.Lookup(model, providerType); ok {
		return info
	}
	return ModelInfo{}
}

// providerPrefixes maps config provider types to the key prefixes used
// in the litellm metadata JSON. Some providers use different prefixes
// (e.g., Google models are keyed as "gemini/" not "google/").
var providerPrefixes = map[string][]string{
	"anthropic":        {"anthropic/"},
	"openai":           {"openai/"},
	"google":           {"gemini/", "google/"},
	"openai_compatible": {}, // no standard prefix
}

// priorityModels lists model name substrings that should sort first,
// in order of priority. Models matching earlier entries sort higher.
var priorityModels = []string{
	"sonnet",
	"opus",
	"haiku",
	"gpt-4o",
	"gpt-4-turbo",
	"o3",
	"o1",
	"gemini-2.5-pro",
	"gemini-2.5-flash",
	"gemini-2.0",
}

// ModelsForProvider returns a sorted list of ModelSummary entries for the
// given provider type. Models are found by matching litellm key prefixes
// (e.g., "anthropic/" for the anthropic provider). The list is sorted with
// priority models first, then alphabetically.
func (s *MetadataStore) ModelsForProvider(providerType string) []config.ModelSummary {
	prefixes := providerPrefixes[providerType]

	seen := make(map[string]bool)
	var results []config.ModelSummary

	for key, info := range s.models {
		matched := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		// Extract model name by stripping the prefix
		modelName := key
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				modelName = strings.TrimPrefix(key, prefix)
				break
			}
		}

		// Skip duplicates (same model name from different prefixes)
		if seen[modelName] {
			continue
		}
		seen[modelName] = true

		results = append(results, config.ModelSummary{
			Name:                    modelName,
			MaxInputTokens:          info.MaxInputTokens,
			SupportsFunctionCalling: info.SupportsFunctionCalling,
			SupportsVision:          info.SupportsVision,
			SupportsReasoning:       info.SupportsReasoning,
		})
	}

	// Sort: priority models first, then alphabetical
	sort.Slice(results, func(i, j int) bool {
		pi := priorityIndex(results[i].Name)
		pj := priorityIndex(results[j].Name)
		if pi != pj {
			return pi < pj
		}
		return results[i].Name < results[j].Name
	})

	return results
}

// priorityIndex returns the sort priority for a model name.
// Lower values sort first. Models not matching any priority pattern
// get a high value (len(priorityModels)) and sort alphabetically after.
func priorityIndex(name string) int {
	lower := strings.ToLower(name)
	for i, pat := range priorityModels {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return i
		}
	}
	return len(priorityModels)
}
