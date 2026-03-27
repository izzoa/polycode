package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/izzoa/polycode/internal/config"
)

// mockMCPServer simulates an MCP stdio server over a net.Conn pair.
// It reads JSON-RPC requests and responds with canned data.
func mockMCPServer(t *testing.T, serverConn net.Conn, tools []map[string]any) {
	t.Helper()
	defer serverConn.Close()

	buf := make([]byte, 4096)
	for {
		n, err := serverConn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			return
		}

		data := buf[:n]
		// The data may contain multiple JSON objects separated by newlines.
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var req map[string]any
			if err := json.Unmarshal([]byte(line), &req); err != nil {
				t.Logf("mock server: failed to parse request: %v", err)
				continue
			}

			method, _ := req["method"].(string)
			id, hasID := req["id"]

			// Notifications have no id.
			if !hasID || id == nil {
				continue
			}

			idNum, _ := id.(float64)

			var result any
			switch method {
			case "initialize":
				result = map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{},
					"serverInfo": map[string]any{
						"name":    "mock-server",
						"version": "1.0.0",
					},
				}
			case "tools/list":
				result = map[string]any{
					"tools": tools,
				}
			case "tools/call":
				params, _ := req["params"].(map[string]any)
				toolName, _ := params["name"].(string)
				result = map[string]any{
					"content": []map[string]any{
						{
							"type": "text",
							"text": fmt.Sprintf("result from %s", toolName),
						},
					},
				}
			default:
				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      idNum,
					"error": map[string]any{
						"code":    -32601,
						"message": "method not found",
					},
				}
				respData, _ := json.Marshal(resp)
				respData = append(respData, '\n')
				serverConn.Write(respData)
				continue
			}

			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      idNum,
				"result":  result,
			}
			respData, _ := json.Marshal(resp)
			respData = append(respData, '\n')
			serverConn.Write(respData)
		}
	}
}

// newTestClient creates an MCPClient with a mock server connected via pipes.
// It returns the client and a cleanup function.
func newTestClient(t *testing.T, serverName string, tools []map[string]any) (*MCPClient, func()) {
	t.Helper()

	// Use a net.Pipe to simulate stdin/stdout of a subprocess.
	clientConn, srvConn := net.Pipe()

	go mockMCPServer(t, srvConn, tools)

	client := &MCPClient{
		servers: make(map[string]*serverConn),
	}

	sc := &serverConn{
		config: config.MCPServerConfig{
			Name: serverName,
		},
		stdin:  clientConn,
		stdout: clientConn,
	}

	// We need to create the scanner from the read side.
	// net.Conn implements both io.ReadCloser and io.WriteCloser.
	// For the serverConn, stdin writes go to the mock and stdout reads come from it.
	// With net.Pipe, both ends are the same conn, which is correct:
	// writing to clientConn sends to srvConn, reading from clientConn reads from srvConn.

	// However, our serverConn expects separate stdin (write) and stdout (read).
	// net.Pipe gives us two connected conns. Let's use two pipes.

	// Actually, let's redo this with two net.Pipe pairs for clarity.
	cleanup := func() {
		clientConn.Close()
	}

	// Close the initial conns since we need two separate pipes.
	clientConn.Close()
	srvConn.Close()

	// Create two pipe pairs: one for stdin, one for stdout.
	clientStdinWriter, serverStdinReader := net.Pipe()
	serverStdoutWriter, clientStdoutReader := net.Pipe()

	go func() {
		defer serverStdinReader.Close()
		defer serverStdoutWriter.Close()

		buf := make([]byte, 4096)
		for {
			n, err := serverStdinReader.Read(buf)
			if err != nil {
				return
			}

			data := buf[:n]
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var req map[string]any
				if err := json.Unmarshal([]byte(line), &req); err != nil {
					t.Logf("mock server: failed to parse: %v", err)
					continue
				}

				method, _ := req["method"].(string)
				id, hasID := req["id"]

				if !hasID || id == nil {
					continue
				}
				idNum, _ := id.(float64)

				var result any
				switch method {
				case "initialize":
					result = map[string]any{
						"protocolVersion": "2024-11-05",
						"capabilities":    map[string]any{},
						"serverInfo": map[string]any{
							"name":    "mock-server",
							"version": "1.0.0",
						},
					}
				case "tools/list":
					result = map[string]any{
						"tools": tools,
					}
				case "tools/call":
					params, _ := req["params"].(map[string]any)
					toolName, _ := params["name"].(string)
					result = map[string]any{
						"content": []map[string]any{
							{
								"type": "text",
								"text": fmt.Sprintf("result from %s", toolName),
							},
						},
					}
				default:
					resp := map[string]any{
						"jsonrpc": "2.0",
						"id":      idNum,
						"error": map[string]any{
							"code":    -32601,
							"message": "method not found",
						},
					}
					respData, _ := json.Marshal(resp)
					respData = append(respData, '\n')
					serverStdoutWriter.Write(respData)
					continue
				}

				resp := map[string]any{
					"jsonrpc": "2.0",
					"id":      idNum,
					"result":  result,
				}
				respData, _ := json.Marshal(resp)
				respData = append(respData, '\n')
				serverStdoutWriter.Write(respData)
			}
		}
	}()

	sc = &serverConn{
		config: config.MCPServerConfig{
			Name: serverName,
		},
		stdin:  clientStdinWriter,
		stdout: clientStdoutReader,
	}

	// Create a bufio.Scanner for reading responses.
	sc.scanner = newLineScanner(clientStdoutReader)

	client.servers[serverName] = sc

	cleanup = func() {
		clientStdinWriter.Close()
		clientStdoutReader.Close()
	}

	return client, cleanup
}

// mockServerData holds canned responses for the mock MCP server.
type mockServerData struct {
	tools     []map[string]any
	resources []map[string]any
	prompts   []map[string]any
}

// newTestClientFull creates an MCPClient with a mock server that supports
// tools/list, resources/list, and prompts/list.
func newTestClientFull(t *testing.T, serverName string, data mockServerData) (*MCPClient, func()) {
	t.Helper()

	clientStdinWriter, serverStdinReader := net.Pipe()
	serverStdoutWriter, clientStdoutReader := net.Pipe()

	go func() {
		defer serverStdinReader.Close()
		defer serverStdoutWriter.Close()

		buf := make([]byte, 4096)
		for {
			n, err := serverStdinReader.Read(buf)
			if err != nil {
				return
			}

			lines := strings.Split(strings.TrimSpace(string(buf[:n])), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				var req map[string]any
				if err := json.Unmarshal([]byte(line), &req); err != nil {
					continue
				}

				method, _ := req["method"].(string)
				id, hasID := req["id"]
				if !hasID || id == nil {
					continue
				}
				idNum, _ := id.(float64)

				var result any
				switch method {
				case "initialize":
					result = map[string]any{
						"protocolVersion": "2024-11-05",
						"capabilities":    map[string]any{},
						"serverInfo":      map[string]any{"name": "mock-server", "version": "1.0.0"},
					}
				case "tools/list":
					result = map[string]any{"tools": data.tools}
				case "resources/list":
					result = map[string]any{"resources": data.resources}
				case "prompts/list":
					result = map[string]any{"prompts": data.prompts}
				case "tools/call":
					params, _ := req["params"].(map[string]any)
					toolName, _ := params["name"].(string)
					result = map[string]any{
						"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("result from %s", toolName)}},
					}
				default:
					resp := map[string]any{"jsonrpc": "2.0", "id": idNum, "error": map[string]any{"code": -32601, "message": "method not found"}}
					respData, _ := json.Marshal(resp)
					respData = append(respData, '\n')
					serverStdoutWriter.Write(respData)
					continue
				}

				resp := map[string]any{"jsonrpc": "2.0", "id": idNum, "result": result}
				respData, _ := json.Marshal(resp)
				respData = append(respData, '\n')
				serverStdoutWriter.Write(respData)
			}
		}
	}()

	sc := &serverConn{
		config:  config.MCPServerConfig{Name: serverName},
		stdin:   clientStdinWriter,
		stdout:  clientStdoutReader,
		scanner: bufio.NewScanner(clientStdoutReader),
	}

	client := &MCPClient{
		servers:      map[string]*serverConn{serverName: sc},
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
	}

	cleanup := func() {
		clientStdinWriter.Close()
		clientStdoutReader.Close()
	}

	return client, cleanup
}

// newLineScanner creates a bufio.Scanner that splits on newlines.
func newLineScanner(r io.Reader) *bufio.Scanner {
	return bufio.NewScanner(r)
}

func TestDiscoverTools(t *testing.T) {
	mockTools := []map[string]any{
		{
			"name":        "read_file",
			"description": "Read a file from the filesystem",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file",
					},
				},
				"required": []string{"path"},
			},
		},
		{
			"name":        "write_file",
			"description": "Write content to a file",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Path to the file",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "Content to write",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}

	client, cleanup := newTestClient(t, "filesystem", mockTools)
	defer cleanup()

	// Run discovery (sends initialize + tools/list to the mock).
	ctx := context.Background()

	// We need to manually send initialize and then tools/list since we
	// bypassed the Connect method.
	conn := client.servers["filesystem"]

	// Send initialize.
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "polycode",
			"version": "1.0.0",
		},
	}
	_, err := conn.sendRequest(context.Background(), "initialize", initParams)
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Discover tools.
	tools, err := client.discoverTools(ctx, "filesystem", conn)
	if err != nil {
		t.Fatalf("discoverTools failed: %v", err)
	}

	// Store discovered tools in client.
	client.mu.Lock()
	client.tools = tools
	client.mu.Unlock()

	// Verify tools.
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	if tools[0].Name != "read_file" {
		t.Errorf("expected tool[0].Name = %q, got %q", "read_file", tools[0].Name)
	}
	if tools[0].Description != "Read a file from the filesystem" {
		t.Errorf("expected tool[0].Description = %q, got %q", "Read a file from the filesystem", tools[0].Description)
	}
	if tools[0].ServerName != "filesystem" {
		t.Errorf("expected tool[0].ServerName = %q, got %q", "filesystem", tools[0].ServerName)
	}

	if tools[1].Name != "write_file" {
		t.Errorf("expected tool[1].Name = %q, got %q", "write_file", tools[1].Name)
	}
	if tools[1].Description != "Write content to a file" {
		t.Errorf("expected tool[1].Description = %q, got %q", "Write content to a file", tools[1].Description)
	}

	// Verify Tools() accessor returns the same tools.
	allTools := client.Tools()
	if len(allTools) != 2 {
		t.Fatalf("Tools() returned %d tools, expected 2", len(allTools))
	}

	// Verify ToToolDefinitions produces correctly prefixed names.
	defs := client.ToToolDefinitions()
	if len(defs) != 2 {
		t.Fatalf("ToToolDefinitions returned %d defs, expected 2", len(defs))
	}
	if defs[0].Name != "mcp_filesystem_read_file" {
		t.Errorf("expected def[0].Name = %q, got %q", "mcp_filesystem_read_file", defs[0].Name)
	}
	if defs[1].Name != "mcp_filesystem_write_file" {
		t.Errorf("expected def[1].Name = %q, got %q", "mcp_filesystem_write_file", defs[1].Name)
	}
}

func TestCallTool(t *testing.T) {
	mockTools := []map[string]any{
		{
			"name":        "greet",
			"description": "Say hello",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	client, cleanup := newTestClient(t, "greeter", mockTools)
	defer cleanup()

	conn := client.servers["greeter"]

	// Initialize first.
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "polycode",
			"version": "1.0.0",
		},
	}
	_, err := conn.sendRequest(context.Background(), "initialize", initParams)
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	ctx := context.Background()

	args := json.RawMessage(`{"name":"world"}`)
	result, err := client.CallTool(ctx, "greeter", "greet", args)
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	expected := "result from greet"
	if result != expected {
		t.Errorf("expected result %q, got %q", expected, result)
	}
}

func TestCallToolUnknownServer(t *testing.T) {
	client := &MCPClient{
		servers: make(map[string]*serverConn),
	}

	ctx := context.Background()
	_, err := client.CallTool(ctx, "nonexistent", "tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown server, got nil")
	}
	if !strings.Contains(err.Error(), "unknown MCP server") {
		t.Errorf("expected 'unknown MCP server' error, got: %v", err)
	}
}

func TestResolveToolCall(t *testing.T) {
	client := &MCPClient{
		servers:   make(map[string]*serverConn),
		toolIndex: make(map[string]struct{ Server, Tool string }),
	}

	// Simulate discovery of tools from a server with underscores in the name.
	client.toolIndex["mcp_my_db_read"] = struct{ Server, Tool string }{"my_db", "read"}
	client.toolIndex["mcp_filesystem_read_file"] = struct{ Server, Tool string }{"filesystem", "read_file"}
	client.toolIndex["mcp_my_db_write_record"] = struct{ Server, Tool string }{"my_db", "write_record"}

	tests := []struct {
		prefixed   string
		wantServer string
		wantTool   string
		wantOK     bool
	}{
		{"mcp_my_db_read", "my_db", "read", true},
		{"mcp_filesystem_read_file", "filesystem", "read_file", true},
		{"mcp_my_db_write_record", "my_db", "write_record", true},
		{"mcp_unknown_tool", "", "", false},
		{"not_mcp", "", "", false},
	}

	for _, tt := range tests {
		server, tool, ok := client.ResolveToolCall(tt.prefixed)
		if ok != tt.wantOK {
			t.Errorf("ResolveToolCall(%q): ok=%v, want %v", tt.prefixed, ok, tt.wantOK)
		}
		if server != tt.wantServer {
			t.Errorf("ResolveToolCall(%q): server=%q, want %q", tt.prefixed, server, tt.wantServer)
		}
		if tool != tt.wantTool {
			t.Errorf("ResolveToolCall(%q): tool=%q, want %q", tt.prefixed, tool, tt.wantTool)
		}
	}
}

func TestToolNameCollisionDetection(t *testing.T) {
	// Server "my_db" with tool "read" and server "my" with tool "db_read"
	// both produce "mcp_my_db_read". Config order determines winner.
	client := &MCPClient{
		configs: []config.MCPServerConfig{
			{Name: "my_db"},
			{Name: "my"},
		},
		servers:      make(map[string]*serverConn),
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
	}

	// Simulate discovered tools from each server (after connectStdio).
	client.tools = nil
	// First server's tools (in config order).
	tool1 := MCPTool{Name: "read", Description: "Read", ServerName: "my_db"}
	prefixed1 := fmt.Sprintf("mcp_%s_%s", tool1.ServerName, tool1.Name)
	client.toolIndex[prefixed1] = struct{ Server, Tool string }{tool1.ServerName, tool1.Name}
	client.tools = append(client.tools, tool1)

	// Second server's tool collides with first.
	tool2 := MCPTool{Name: "db_read", Description: "DB Read", ServerName: "my"}
	prefixed2 := fmt.Sprintf("mcp_%s_%s", tool2.ServerName, tool2.Name)

	// Apply same collision logic as Connect/reconnectServer.
	if existing, ok := client.toolIndex[prefixed2]; ok && existing.Server != tool2.ServerName {
		// Collision detected — skip from both toolIndex and tools.
		// Verify first server wins.
		server, _, ok := client.ResolveToolCall(prefixed2)
		if !ok {
			t.Fatal("expected ResolveToolCall to succeed for existing tool")
		}
		if server != "my_db" {
			t.Errorf("expected first server 'my_db' to win collision, got %q", server)
		}
	} else {
		t.Fatal("expected collision to be detected for prefixed name: " + prefixed2)
	}

	// Verify c.tools only has the first server's tool (collision was not appended).
	if len(client.tools) != 1 {
		t.Errorf("expected 1 tool after collision, got %d", len(client.tools))
	}
	if client.tools[0].ServerName != "my_db" {
		t.Errorf("expected remaining tool to be from 'my_db', got %q", client.tools[0].ServerName)
	}

	// Verify ToToolDefinitions only has 1 entry (no duplicate).
	defs := client.ToToolDefinitions()
	if len(defs) != 1 {
		t.Errorf("expected 1 tool definition, got %d", len(defs))
	}
}

func TestToToolDefinitionsAndResolveConsistency(t *testing.T) {
	// Verify that ToToolDefinitions and ResolveToolCall are consistent.
	client := &MCPClient{
		servers:   make(map[string]*serverConn),
		toolIndex: make(map[string]struct{ Server, Tool string }),
		tools: []MCPTool{
			{Name: "read", Description: "Read", ServerName: "my_db"},
			{Name: "write_record", Description: "Write", ServerName: "my_db"},
			{Name: "read_file", Description: "Read file", ServerName: "filesystem"},
		},
	}

	// Populate toolIndex the same way Connect() does.
	for _, t := range client.tools {
		prefixed := fmt.Sprintf("mcp_%s_%s", t.ServerName, t.Name)
		client.toolIndex[prefixed] = struct{ Server, Tool string }{t.ServerName, t.Name}
	}

	defs := client.ToToolDefinitions()
	for _, def := range defs {
		server, tool, ok := client.ResolveToolCall(def.Name)
		if !ok {
			t.Errorf("ResolveToolCall(%q) returned ok=false for a tool from ToToolDefinitions", def.Name)
			continue
		}
		// Verify the resolved names are valid
		if server == "" || tool == "" {
			t.Errorf("ResolveToolCall(%q) returned empty server=%q or tool=%q", def.Name, server, tool)
		}
	}
}

func TestRemoveServerMetadata(t *testing.T) {
	client := &MCPClient{
		servers:      make(map[string]*serverConn),
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
		tools: []MCPTool{
			{Name: "read", ServerName: "fs"},
			{Name: "write", ServerName: "fs"},
			{Name: "query", ServerName: "db"},
		},
		resources: []MCPResource{
			{Name: "file.txt", ServerName: "fs"},
			{Name: "schema", ServerName: "db"},
		},
		prompts: []MCPPrompt{
			{Name: "explain", ServerName: "fs"},
			{Name: "summarize", ServerName: "db"},
		},
	}
	client.toolIndex["mcp_fs_read"] = struct{ Server, Tool string }{"fs", "read"}
	client.toolIndex["mcp_fs_write"] = struct{ Server, Tool string }{"fs", "write"}
	client.toolIndex["mcp_db_query"] = struct{ Server, Tool string }{"db", "query"}

	// Remove "fs" metadata.
	client.mu.Lock()
	client.removeServerMetadata("fs")
	client.mu.Unlock()

	// Tools: only "db" should remain.
	if len(client.tools) != 1 || client.tools[0].ServerName != "db" {
		t.Errorf("expected 1 tool for db, got %d tools: %+v", len(client.tools), client.tools)
	}

	// ToolIndex: only db entry should remain.
	if _, ok := client.toolIndex["mcp_fs_read"]; ok {
		t.Error("mcp_fs_read should have been removed from toolIndex")
	}
	if _, ok := client.toolIndex["mcp_db_query"]; !ok {
		t.Error("mcp_db_query should still be in toolIndex")
	}

	// Resources: only "db" should remain.
	if len(client.resources) != 1 || client.resources[0].ServerName != "db" {
		t.Errorf("expected 1 resource for db, got %d: %+v", len(client.resources), client.resources)
	}

	// Prompts: only "db" should remain.
	if len(client.prompts) != 1 || client.prompts[0].ServerName != "db" {
		t.Errorf("expected 1 prompt for db, got %d: %+v", len(client.prompts), client.prompts)
	}
}

func TestDisconnectServerCleansAllMetadata(t *testing.T) {
	client := &MCPClient{
		servers:      make(map[string]*serverConn),
		serverErrors: make(map[string]error),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
		tools: []MCPTool{
			{Name: "read", ServerName: "fs"},
			{Name: "query", ServerName: "db"},
		},
		resources: []MCPResource{
			{Name: "file.txt", ServerName: "fs"},
			{Name: "schema", ServerName: "db"},
		},
		prompts: []MCPPrompt{
			{Name: "explain", ServerName: "fs"},
		},
	}
	client.toolIndex["mcp_fs_read"] = struct{ Server, Tool string }{"fs", "read"}
	client.toolIndex["mcp_db_query"] = struct{ Server, Tool string }{"db", "query"}
	client.serverErrors["fs"] = fmt.Errorf("some error")

	client.DisconnectServer("fs")

	// Verify tools.
	if len(client.tools) != 1 || client.tools[0].Name != "query" {
		t.Errorf("expected only db tool, got: %+v", client.tools)
	}

	// Verify resources.
	if len(client.resources) != 1 || client.resources[0].Name != "schema" {
		t.Errorf("expected only db resource, got: %+v", client.resources)
	}

	// Verify prompts.
	if len(client.prompts) != 0 {
		t.Errorf("expected no prompts (fs was the only one with prompts), got: %+v", client.prompts)
	}

	// Verify toolIndex.
	if _, ok := client.toolIndex["mcp_fs_read"]; ok {
		t.Error("mcp_fs_read should be gone from toolIndex")
	}

	// Verify serverErrors cleaned.
	if _, ok := client.serverErrors["fs"]; ok {
		t.Error("serverErrors for fs should have been cleaned")
	}
}

func TestResourcesAndPromptsAccessors(t *testing.T) {
	client := &MCPClient{
		servers:     make(map[string]*serverConn),
		toolIndex:   make(map[string]struct{ Server, Tool string }),
		reconnectMu: make(map[string]*sync.Mutex),
		resources: []MCPResource{
			{Name: "file.txt", URI: "file:///tmp/file.txt", ServerName: "fs"},
		},
		prompts: []MCPPrompt{
			{Name: "explain", Description: "Explain code", ServerName: "fs"},
		},
	}

	// Resources returns a copy.
	resources := client.Resources()
	if len(resources) != 1 || resources[0].Name != "file.txt" {
		t.Errorf("Resources() returned unexpected: %+v", resources)
	}

	// Prompts returns a copy.
	prompts := client.Prompts()
	if len(prompts) != 1 || prompts[0].Name != "explain" {
		t.Errorf("Prompts() returned unexpected: %+v", prompts)
	}

	// Mutating the returned slices doesn't affect the client.
	resources[0].Name = "mutated"
	if client.resources[0].Name != "file.txt" {
		t.Error("Resources() should return a copy, not a reference")
	}
}

func TestNewMCPClient(t *testing.T) {
	configs := []config.MCPServerConfig{
		{Name: "test", Command: "echo", Args: []string{"hello"}},
	}
	client := NewMCPClient(configs, false)
	if client == nil {
		t.Fatal("NewMCPClient returned nil")
	}
	if len(client.configs) != 1 {
		t.Errorf("expected 1 config, got %d", len(client.configs))
	}
	if len(client.servers) != 0 {
		t.Errorf("expected 0 servers before Connect, got %d", len(client.servers))
	}
}

func TestMultiplexedReader_ConcurrentRequests(t *testing.T) {
	tools := []map[string]any{
		{"name": "tool_a", "description": "A", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
		{"name": "tool_b", "description": "B", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	}

	client, cleanup := newTestClientFull(t, "mux", mockServerData{tools: tools})
	defer cleanup()

	conn := client.servers["mux"]

	// Initialize.
	_, err := conn.sendRequest(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Send two concurrent tools/call requests with different tool names.
	// The mock responds with "result from {toolName}", so we can verify
	// each goroutine receives the correct response (not misrouted).
	type callResult struct {
		toolName string
		output   string
		err      error
	}
	results := make(chan callResult, 2)

	for _, name := range []string{"tool_a", "tool_b"} {
		go func(tn string) {
			params := map[string]any{"name": tn}
			resp, err := conn.sendRequest(context.Background(), "tools/call", params)
			if err != nil {
				results <- callResult{toolName: tn, err: err}
				return
			}
			var result toolCallResult
			if err := json.Unmarshal(resp, &result); err != nil {
				results <- callResult{toolName: tn, err: err}
				return
			}
			text := ""
			for _, c := range result.Content {
				if c.Type == "text" {
					text = c.Text
				}
			}
			results <- callResult{toolName: tn, output: text}
		}(name)
	}

	for range 2 {
		r := <-results
		if r.err != nil {
			t.Fatalf("concurrent call to %s failed: %v", r.toolName, r.err)
		}
		expected := "result from " + r.toolName
		if r.output != expected {
			t.Errorf("misrouted response for %s: expected %q, got %q", r.toolName, expected, r.output)
		}
	}
}

func TestMultiplexedReader_NotificationDispatch(t *testing.T) {
	client, cleanup := newTestClientFull(t, "notify", mockServerData{})
	defer cleanup()

	conn := client.servers["notify"]

	// Set up notification handler BEFORE the first sendRequest so the
	// multiplexed reader starts with the handler already registered.
	notifCh := make(chan string, 1)
	conn.onNotify = func(method string, params json.RawMessage) {
		notifCh <- method
	}

	// Initialize — this triggers startReader() with onNotify in place.
	_, err := conn.sendRequest(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	// Verify the reader is running and processes requests correctly with
	// a notification handler registered.
	_, err = conn.sendRequest(context.Background(), "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list failed: %v", err)
	}

	// Note: the mock server doesn't send unsolicited notifications, so we
	// cannot test notification delivery here. This test validates that the
	// reader starts correctly with onNotify registered and does not deadlock
	// or race when processing normal request/response traffic alongside a
	// notification handler.
}

func TestMcpConfigChanged(t *testing.T) {
	base := config.MCPServerConfig{
		Name: "fs", Command: "npx", Args: []string{"-y", "server"},
		Env: map[string]string{"KEY": "val"}, ReadOnly: false, Timeout: 30,
	}

	tests := []struct {
		name    string
		modify  func(config.MCPServerConfig) config.MCPServerConfig
		changed bool
	}{
		{"identical", func(c config.MCPServerConfig) config.MCPServerConfig { return c }, false},
		{"command changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.Command = "node"; return c }, true},
		{"args changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.Args = []string{"-y", "other"}; return c }, true},
		{"url changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.URL = "http://x"; return c }, true},
		{"env changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.Env = map[string]string{"KEY": "new"}; return c }, true},
		{"readonly changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.ReadOnly = true; return c }, true},
		{"timeout changed", func(c config.MCPServerConfig) config.MCPServerConfig { c.Timeout = 60; return c }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modified := tt.modify(base)
			got := mcpConfigChanged(base, modified)
			if got != tt.changed {
				t.Errorf("mcpConfigChanged: got %v, want %v", got, tt.changed)
			}
		})
	}
}

func TestDiscoverResources(t *testing.T) {
	resources := []map[string]any{
		{"uri": "file:///tmp/a.txt", "name": "a.txt", "description": "File A", "mimeType": "text/plain"},
		{"uri": "file:///tmp/b.json", "name": "b.json", "description": "File B", "mimeType": "application/json"},
	}

	client, cleanup := newTestClientFull(t, "fs", mockServerData{resources: resources})
	defer cleanup()

	conn := client.servers["fs"]
	// Initialize first.
	_, err := conn.sendRequest(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	result := client.discoverResources(context.Background(), "fs", conn)
	if len(result) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result))
	}
	if result[0].Name != "a.txt" || result[0].URI != "file:///tmp/a.txt" {
		t.Errorf("unexpected resource[0]: %+v", result[0])
	}
	if result[1].ServerName != "fs" {
		t.Errorf("expected ServerName 'fs', got %q", result[1].ServerName)
	}
}

func TestDiscoverPrompts(t *testing.T) {
	prompts := []map[string]any{
		{
			"name":        "explain",
			"description": "Explain code",
			"arguments": []map[string]any{
				{"name": "code", "description": "Code to explain", "required": true},
				{"name": "language", "description": "Language", "required": false},
			},
		},
	}

	client, cleanup := newTestClientFull(t, "ai", mockServerData{prompts: prompts})
	defer cleanup()

	conn := client.servers["ai"]
	_, err := conn.sendRequest(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	result := client.discoverPrompts(context.Background(), "ai", conn)
	if len(result) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(result))
	}
	if result[0].Name != "explain" {
		t.Errorf("expected prompt name 'explain', got %q", result[0].Name)
	}
	if len(result[0].Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(result[0].Arguments))
	}
	if !result[0].Arguments[0].Required {
		t.Error("expected first argument to be required")
	}
	if result[0].Arguments[1].Required {
		t.Error("expected second argument to not be required")
	}
}

func TestStatus(t *testing.T) {
	tools := []map[string]any{
		{"name": "read", "description": "Read", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	}
	client, cleanup := newTestClientFull(t, "fs", mockServerData{tools: tools})
	defer cleanup()

	// Manually populate tools to simulate a connected server.
	client.tools = []MCPTool{{Name: "read", ServerName: "fs"}}
	client.configs = []config.MCPServerConfig{{Name: "fs", Command: "echo"}}

	statuses := client.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Connected {
		t.Error("expected server to be connected")
	}
	if statuses[0].ToolCount != 1 {
		t.Errorf("expected 1 tool, got %d", statuses[0].ToolCount)
	}
}

func TestClose(t *testing.T) {
	client, cleanup := newTestClientFull(t, "fs", mockServerData{})
	defer cleanup()

	client.tools = []MCPTool{{Name: "read", ServerName: "fs"}}
	client.resources = []MCPResource{{Name: "file", ServerName: "fs"}}
	client.prompts = []MCPPrompt{{Name: "explain", ServerName: "fs"}}

	err := client.Close()
	if err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if len(client.servers) != 0 {
		t.Errorf("expected 0 servers after Close, got %d", len(client.servers))
	}
	if client.tools != nil {
		t.Errorf("expected nil tools after Close, got %d", len(client.tools))
	}
}

func TestCallCount(t *testing.T) {
	tools := []map[string]any{
		{"name": "greet", "description": "Say hello", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	}
	client, cleanup := newTestClientFull(t, "greeter", mockServerData{tools: tools})
	defer cleanup()

	conn := client.servers["greeter"]
	_, err := conn.sendRequest(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("initialize failed: %v", err)
	}

	if client.CallCount() != 0 {
		t.Errorf("expected 0 calls initially, got %d", client.CallCount())
	}

	_, _ = client.CallTool(context.Background(), "greeter", "greet", nil)
	_, _ = client.CallTool(context.Background(), "greeter", "greet", nil)

	if client.CallCount() != 2 {
		t.Errorf("expected 2 calls, got %d", client.CallCount())
	}
}

func TestReadOnlyToolDefinitions(t *testing.T) {
	client := &MCPClient{
		servers:      make(map[string]*serverConn),
		toolIndex:    make(map[string]struct{ Server, Tool string }),
		reconnectMu:  make(map[string]*sync.Mutex),
		tools: []MCPTool{
			{Name: "read", Description: "Read", ServerName: "fs", ReadOnly: true},
			{Name: "write", Description: "Write", ServerName: "fs", ReadOnly: false},
			{Name: "query", Description: "Query", ServerName: "db", ReadOnly: true},
		},
	}

	defs := client.ReadOnlyToolDefinitions()
	if len(defs) != 2 {
		t.Fatalf("expected 2 read-only tool definitions, got %d", len(defs))
	}
	// Verify both returned are read-only tools.
	names := map[string]bool{}
	for _, d := range defs {
		names[d.Name] = true
	}
	if !names["mcp_fs_read"] {
		t.Error("expected mcp_fs_read in read-only definitions")
	}
	if !names["mcp_db_query"] {
		t.Error("expected mcp_db_query in read-only definitions")
	}
	if names["mcp_fs_write"] {
		t.Error("mcp_fs_write should NOT be in read-only definitions")
	}
}
