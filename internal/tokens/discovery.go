package tokens

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

// modelsResponse is the standard OpenAI /v1/models response format.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// DiscoverModels queries an OpenAI-compatible /models endpoint to list
// available models. It tries {baseURL}/models first; if that returns 404
// and baseURL doesn't already end with /v1, it retries {baseURL}/v1/models.
// Returns sorted model IDs on success.
func DiscoverModels(baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimRight(baseURL, "/")

	ids, err := fetchModels(baseURL+"/models", apiKey)
	if err != nil && !strings.HasSuffix(baseURL, "/v1") {
		// Retry with /v1 prefix
		ids, err = fetchModels(baseURL+"/v1/models", apiKey)
	}
	if err != nil {
		return nil, err
	}

	sort.Strings(ids)
	return ids, nil
}

func fetchModels(url, apiKey string) ([]string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying models endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("models endpoint returned 404")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading models response: %w", err)
	}

	var result modelsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing models response: %w", err)
	}

	var ids []string
	for _, m := range result.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}

// EnrichWithMetadata cross-references discovered model IDs against the litellm
// MetadataStore to attach capability information. Models without a litellm match
// are included with zero-value capabilities.
func EnrichWithMetadata(modelIDs []string, store *MetadataStore) []config.ModelSummary {
	results := make([]config.ModelSummary, 0, len(modelIDs))
	for _, id := range modelIDs {
		ms := config.ModelSummary{Name: id}
		if store != nil {
			if info, ok := store.Lookup(id, ""); ok {
				ms.MaxInputTokens = info.MaxInputTokens
				ms.SupportsFunctionCalling = info.SupportsFunctionCalling
				ms.SupportsVision = info.SupportsVision
				ms.SupportsReasoning = info.SupportsReasoning
			}
		}
		results = append(results, ms)
	}
	return results
}
