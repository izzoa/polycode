package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/izzoa/polycode/internal/config"
)

const (
	defaultRegistryURL = "https://registry.modelcontextprotocol.io"
	registryCacheTTL   = 15 * time.Minute
	registryTimeout    = 5 * time.Second
	registryPageLimit  = 20
)

// RegistryServer represents a server from the MCP Registry API.
type RegistryServer struct {
	Name        string
	Description string
	Version     string
	Packages    []RegistryPackage
	Remotes     []RegistryRemote
	Repository  string // GitHub URL
}

// RegistryPackage represents an installable package for a registry server.
type RegistryPackage struct {
	RegistryType string // "npm", "pip", "oci"
	Identifier   string // e.g., "@modelcontextprotocol/server-github"
	Transport    string // "stdio", "streamable-http"
	EnvVars      []RegistryEnvVar
}

// RegistryEnvVar represents an environment variable required by a package.
type RegistryEnvVar struct {
	Name        string
	Description string
	IsRequired  bool
	IsSecret    bool
}

// RegistryRemote represents a hosted HTTP endpoint for a registry server.
type RegistryRemote struct {
	Type string // "streamable-http"
	URL  string
}

// RegistryClient queries the official MCP Registry API.
type RegistryClient struct {
	baseURL    string
	httpClient *http.Client
	cache      map[string]cachedResult
	cacheMu    sync.Mutex
}

type cachedResult struct {
	servers    []RegistryServer
	nextCursor string
	fetchedAt  time.Time
}

// registryAPIResponse is the raw JSON response from /v0/servers.
type registryAPIResponse struct {
	Servers []struct {
		Server struct {
			Name        string `json:"name"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Packages    []struct {
				RegistryType string `json:"registryType"`
				Identifier   string `json:"identifier"`
				Transport    struct {
					Type string `json:"type"`
				} `json:"transport"`
				EnvironmentVariables []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					IsRequired  bool   `json:"isRequired"`
					IsSecret    bool   `json:"isSecret"`
				} `json:"environmentVariables"`
			} `json:"packages"`
			Remotes []struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"remotes"`
			Repository struct {
				URL string `json:"url"`
			} `json:"repository"`
		} `json:"server"`
	} `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

// NewRegistryClient creates a client for the official MCP Registry.
func NewRegistryClient() *RegistryClient {
	return &RegistryClient{
		baseURL: defaultRegistryURL,
		httpClient: &http.Client{
			Timeout: registryTimeout,
		},
		cache: make(map[string]cachedResult),
	}
}

// Search queries the registry for servers matching the given query.
// Returns servers, a cursor for pagination (empty if no more), and any error.
func (rc *RegistryClient) Search(ctx context.Context, query string, limit int) ([]RegistryServer, string, error) {
	return rc.searchWithCursor(ctx, query, limit, "")
}

// SearchNext fetches the next page of results using the cursor from a previous search.
func (rc *RegistryClient) SearchNext(ctx context.Context, query string, limit int, cursor string) ([]RegistryServer, string, error) {
	return rc.searchWithCursor(ctx, query, limit, cursor)
}

func (rc *RegistryClient) searchWithCursor(ctx context.Context, query string, limit int, cursor string) ([]RegistryServer, string, error) {
	if limit <= 0 {
		limit = registryPageLimit
	}

	// Check cache (only for first page — paginated requests bypass cache).
	cacheKey := fmt.Sprintf("%s:%d", query, limit)
	if cursor == "" {
		rc.cacheMu.Lock()
		if cached, ok := rc.cache[cacheKey]; ok && time.Since(cached.fetchedAt) < registryCacheTTL {
			rc.cacheMu.Unlock()
			return cached.servers, cached.nextCursor, nil
		}
		rc.cacheMu.Unlock()
	}

	// Build URL.
	u, _ := url.Parse(rc.baseURL + "/v0/servers")
	q := u.Query()
	if query != "" {
		q.Set("search", query)
	}
	q.Set("limit", fmt.Sprintf("%d", limit))
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating registry request: %w", err)
	}
	req.Header.Set("User-Agent", "polycode")

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("registry returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp registryAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, "", fmt.Errorf("parsing registry response: %w", err)
	}

	servers := parseRegistryResponse(apiResp)
	nextCursor := apiResp.Metadata.NextCursor

	// Cache first-page results.
	if cursor == "" {
		rc.cacheMu.Lock()
		rc.cache[cacheKey] = cachedResult{
			servers:    servers,
			nextCursor: nextCursor,
			fetchedAt:  time.Now(),
		}
		rc.cacheMu.Unlock()
	}

	return servers, nextCursor, nil
}

func parseRegistryResponse(resp registryAPIResponse) []RegistryServer {
	var servers []RegistryServer
	for _, entry := range resp.Servers {
		s := entry.Server
		rs := RegistryServer{
			Name:        s.Name,
			Description: s.Description,
			Version:     s.Version,
			Repository:  s.Repository.URL,
		}
		// Use title if description is empty.
		if rs.Description == "" && s.Title != "" {
			rs.Description = s.Title
		}

		for _, p := range s.Packages {
			pkg := RegistryPackage{
				RegistryType: p.RegistryType,
				Identifier:   p.Identifier,
				Transport:    p.Transport.Type,
			}
			for _, ev := range p.EnvironmentVariables {
				pkg.EnvVars = append(pkg.EnvVars, RegistryEnvVar{
					Name:        ev.Name,
					Description: ev.Description,
					IsRequired:  ev.IsRequired,
					IsSecret:    ev.IsSecret,
				})
			}
			rs.Packages = append(rs.Packages, pkg)
		}

		for _, r := range s.Remotes {
			rs.Remotes = append(rs.Remotes, RegistryRemote{
				Type: r.Type,
				URL:  r.URL,
			})
		}

		servers = append(servers, rs)
	}
	return servers
}

// EnvVarMeta holds metadata about an environment variable needed by an MCP server.
// Used by CLI/TUI to decide masking and keyring storage.
type EnvVarMeta struct {
	Name        string
	Description string
	IsSecret    bool
	IsRequired  bool
}

// ToMCPServerConfig maps a registry server to an MCPServerConfig and returns
// env var metadata for input masking and keyring decisions.
func ToMCPServerConfig(server RegistryServer) (config.MCPServerConfig, []EnvVarMeta) {
	cfg := config.MCPServerConfig{
		Name: deriveServerName(server.Name),
	}
	var envMeta []EnvVarMeta

	// Prefer packages (local install) over remotes (hosted).
	mapped := false
	if len(server.Packages) > 0 {
		pkg := server.Packages[0]
		switch pkg.RegistryType {
		case "npm":
			cfg.Command = "npx"
			cfg.Args = []string{"-y", pkg.Identifier}
			mapped = true
		case "pip", "pypi":
			cfg.Command = "uvx"
			cfg.Args = []string{pkg.Identifier}
			mapped = true
		case "oci":
			cfg.Command = "docker"
			cfg.Args = []string{"run", "--rm", "-i", pkg.Identifier}
			mapped = true
		}

		// Pre-populate env vars and collect metadata.
		if len(pkg.EnvVars) > 0 {
			cfg.Env = make(map[string]string)
			for _, ev := range pkg.EnvVars {
				cfg.Env[ev.Name] = ""
				envMeta = append(envMeta, EnvVarMeta{
					Name:        ev.Name,
					Description: ev.Description,
					IsSecret:    ev.IsSecret,
					IsRequired:  ev.IsRequired,
				})
			}
		}
	}

	// Fall back to remote if no package was mapped.
	if !mapped && len(server.Remotes) > 0 {
		cfg.URL = server.Remotes[0].URL
	}

	return cfg, envMeta
}

// deriveServerName extracts a short name from a registry name like "ai.smithery/github-mcp".
func deriveServerName(registryName string) string {
	if idx := strings.LastIndex(registryName, "/"); idx >= 0 && idx < len(registryName)-1 {
		return registryName[idx+1:]
	}
	// Fallback: use the full name, replacing dots/slashes.
	name := strings.ReplaceAll(registryName, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return name
}

// TransportLabel returns a human-readable transport label for display.
func (rs RegistryServer) TransportLabel() string {
	if len(rs.Packages) > 0 {
		pkg := rs.Packages[0]
		return pkg.RegistryType + "/" + pkg.Transport
	}
	if len(rs.Remotes) > 0 {
		return rs.Remotes[0].Type
	}
	return "unknown"
}

// PackageIdentifier returns the first package identifier, or the remote URL.
func (rs RegistryServer) PackageIdentifier() string {
	if len(rs.Packages) > 0 {
		return rs.Packages[0].Identifier
	}
	if len(rs.Remotes) > 0 {
		return rs.Remotes[0].URL
	}
	return ""
}
