package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// httpConn implements the MCP streamable HTTP transport.
// Instead of communicating via stdio pipes, it sends JSON-RPC requests
// as HTTP POSTs to a server URL.
type httpConn struct {
	url        string
	serverName string // for logging
	sessionID  string // Mcp-Session-Id header from server
	client     *http.Client
	nextID     atomic.Int64
	mu         sync.Mutex
	dead       atomic.Bool
	debug      *debugLogger
}

// sendRequestHTTP sends a JSON-RPC request via HTTP POST and returns the result.
func (hc *httpConn) sendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.dead.Load() {
		hc.debug.LogResponse(hc.serverName, method, "", "HTTP connection is dead")
		return nil, fmt.Errorf("HTTP connection is dead")
	}

	id := hc.nextID.Add(1)

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(req)
	if err != nil {
		hc.debug.LogResponse(hc.serverName, method, "", err.Error())
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	hc.debug.LogRequest(hc.serverName, method, string(body))

	httpReq, err := http.NewRequestWithContext(ctx, "POST", hc.url, bytes.NewReader(body))
	if err != nil {
		hc.debug.LogResponse(hc.serverName, method, "", err.Error())
		return nil, fmt.Errorf("creating HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if hc.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", hc.sessionID)
	}

	httpResp, err := hc.client.Do(httpReq)
	if err != nil {
		hc.dead.Store(true)
		hc.debug.LogResponse(hc.serverName, method, "", err.Error())
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Extract session ID from response headers.
	if sid := httpResp.Header.Get("Mcp-Session-Id"); sid != "" {
		hc.sessionID = sid
	}

	if httpResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(httpResp.Body)
		errMsg := fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, string(respBody))
		hc.debug.LogResponse(hc.serverName, method, "", errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Dispatch based on Content-Type: SSE stream or plain JSON.
	contentType := httpResp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		result, err := parseSSEResponse(httpResp.Body)
		if err != nil {
			hc.debug.LogResponse(hc.serverName, method, "", err.Error())
			return nil, err
		}
		hc.debug.LogResponse(hc.serverName, method, string(result), "")
		return result, nil
	}

	// Plain JSON response (default path).
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		hc.debug.LogResponse(hc.serverName, method, "", err.Error())
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var resp jsonrpcResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		hc.debug.LogResponse(hc.serverName, method, "", err.Error())
		return nil, fmt.Errorf("parsing JSON-RPC response: %w", err)
	}

	if resp.Error != nil {
		errMsg := fmt.Sprintf("server error %d: %s", resp.Error.Code, resp.Error.Message)
		hc.debug.LogResponse(hc.serverName, method, "", errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	hc.debug.LogResponse(hc.serverName, method, string(resp.Result), "")
	return resp.Result, nil
}

// parseSSEResponse reads an SSE (Server-Sent Events) stream and extracts
// the JSON-RPC response. Per the SSE spec, events are separated by blank
// lines. Multiple `data:` lines within one event are joined with "\n".
// Returns the last complete JSON-RPC result found in the stream.
func parseSSEResponse(body io.Reader) (json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
	// Increase buffer for large JSON payloads (1 MiB).
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lastResult json.RawMessage
	var lastErr error
	var dataLines []string // buffered data: lines for current event

	// tryParseEvent attempts to parse the buffered data lines as a JSON-RPC response.
	tryParseEvent := func() {
		if len(dataLines) == 0 {
			return
		}
		data := strings.Join(dataLines, "\n")
		dataLines = nil

		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			return // not a JSON-RPC message
		}
		if resp.Error != nil {
			lastErr = fmt.Errorf("server error %d: %s", resp.Error.Code, resp.Error.Message)
			lastResult = nil
		} else if resp.Result != nil {
			lastResult = resp.Result
			lastErr = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Blank line = event separator per SSE spec.
		if line == "" {
			tryParseEvent()
			continue
		}

		// Skip event type prefixes and id fields.
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, ":") {
			continue
		}

		// Buffer data payload lines.
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimPrefix(line, "data:")
			if len(payload) > 0 && payload[0] == ' ' {
				payload = payload[1:] // SSE spec: single leading space is stripped
			}
			dataLines = append(dataLines, payload)
		}
	}

	// Handle final event if stream ends without trailing blank line.
	tryParseEvent()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading SSE stream: %w", err)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	if lastResult != nil {
		return lastResult, nil
	}
	return nil, fmt.Errorf("no JSON-RPC response found in SSE stream")
}

// close marks the HTTP connection as dead. HTTP connections don't hold
// persistent resources, so this is a lightweight operation.
func (hc *httpConn) close() error {
	hc.dead.Store(true)
	return nil
}

// isAlive returns true if the HTTP connection hasn't been marked dead.
func (hc *httpConn) isAlive() bool {
	return !hc.dead.Load()
}
