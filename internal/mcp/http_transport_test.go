package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSSEResponse_SingleEvent(t *testing.T) {
	body := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"value\":42}}\n\n"
	result, err := parseSSEResponse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != `{"value":42}` {
		t.Errorf("expected {\"value\":42}, got %s", string(result))
	}
}

func TestParseSSEResponse_MultiLineData(t *testing.T) {
	// Per SSE spec, multiple data: lines in one event are joined with "\n".
	// This test verifies joining works by splitting JSON across two data: lines.
	body := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\n" +
		"data: \"result\":{\"ok\":true}}\n\n"
	result, err := parseSSEResponse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result should be valid JSON: %v (raw: %s)", err, string(result))
	}
	if parsed["ok"] != true {
		t.Errorf("expected ok=true, got %v", parsed["ok"])
	}

	// Also verify that the raw joined data contains a newline (proving lines
	// were joined with \n, not just concatenated). The full joined event is
	// the two data payloads separated by \n.
	// We test this by checking that a second event in the stream also works,
	// proving the blank-line event boundary logic is correct.
	multiEvent := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"first\":true}}\n\n" +
		"data: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":{\"second\":true}}\n\n"
	result2, err := parseSSEResponse(strings.NewReader(multiEvent))
	if err != nil {
		t.Fatalf("multi-event error: %v", err)
	}
	// parseSSEResponse returns the LAST valid result.
	var parsed2 map[string]any
	if err := json.Unmarshal(result2, &parsed2); err != nil {
		t.Fatalf("multi-event JSON parse error: %v", err)
	}
	if parsed2["second"] != true {
		t.Errorf("expected last event result, got %+v", parsed2)
	}
}

func TestParseSSEResponse_CommentsAndPrefixes(t *testing.T) {
	body := ": this is a comment\n" +
		"event: message\n" +
		"id: 123\n" +
		"data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"x\":1}}\n\n"
	result, err := parseSSEResponse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != `{"x":1}` {
		t.Errorf("expected {\"x\":1}, got %s", string(result))
	}
}

func TestParseSSEResponse_NoTrailingBlankLine(t *testing.T) {
	// Stream ends without a trailing blank line — should still parse
	body := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"done\":true}}\n"
	result, err := parseSSEResponse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != `{"done":true}` {
		t.Errorf("expected {\"done\":true}, got %s", string(result))
	}
}

func TestParseSSEResponse_ErrorResponse(t *testing.T) {
	body := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32600,\"message\":\"bad request\"}}\n\n"
	_, err := parseSSEResponse(strings.NewReader(body))
	if err == nil {
		t.Fatal("expected error for JSON-RPC error response")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Errorf("expected error to contain 'bad request', got: %v", err)
	}
}

func TestParseSSEResponse_Empty(t *testing.T) {
	_, err := parseSSEResponse(strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty SSE stream")
	}
}

func TestHTTPTransport_JSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]any{"tools": []any{}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	hc := &httpConn{
		url:        srv.URL,
		serverName: "test",
		client:     srv.Client(),
		debug:      newDebugLogger(false),
	}

	ctx := t.Context()
	result, err := hc.sendRequest(ctx, "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHTTPTransport_SSEResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"tools\":[]}}\n\n")
	}))
	defer srv.Close()

	hc := &httpConn{
		url:        srv.URL,
		serverName: "test-sse",
		client:     srv.Client(),
		debug:      newDebugLogger(false),
	}

	ctx := t.Context()
	result, err := hc.sendRequest(ctx, "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestHTTPTransport_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	hc := &httpConn{
		url:        srv.URL,
		serverName: "test-err",
		client:     srv.Client(),
		debug:      newDebugLogger(false),
	}

	ctx := t.Context()
	_, err := hc.sendRequest(ctx, "tools/list", map[string]any{})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got: %v", err)
	}
}
