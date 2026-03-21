package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
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
	_, err := conn.sendRequest("initialize", initParams)
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
	_, err := conn.sendRequest("initialize", initParams)
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

func TestNewMCPClient(t *testing.T) {
	configs := []config.MCPServerConfig{
		{Name: "test", Command: "echo", Args: []string{"hello"}},
	}
	client := NewMCPClient(configs)
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
