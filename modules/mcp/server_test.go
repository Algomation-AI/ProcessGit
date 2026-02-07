// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestToolContext() *ToolContext {
	return &ToolContext{
		Config: &MCPConfig{
			Version: 1,
			Server: MCPServerConfig{
				Name:        "Test Server",
				Description: "A test server",
			},
			Sources: []MCPSource{
				{Path: "test.xml", Type: "xml"},
			},
		},
		Index: &EntityIndex{
			Entities: map[string]*Entity{
				"item:01": {
					ID:         "item:01",
					Type:       "item",
					Name:       "Test Item",
					Attributes: map[string]string{"code": "01", "value": "hello"},
				},
			},
			ByType:   map[string][]string{"item": {"item:01"}},
			ByParent: make(map[string][]string),
			Stats:    IndexStats{TotalEntities: 1, TypeCounts: map[string]int{"item": 1}},
		},
	}
}

func TestHandleJSONRPC_Initialize(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
	assert.Equal(t, float64(1), resp.ID)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(InitializeResult)
	require.True(t, ok)
	assert.Equal(t, MCPProtocolVersion, result.ProtocolVersion)
	assert.Equal(t, "Test Server", result.ServerInfo.Name)
	assert.NotNil(t, result.Capabilities.Tools)
}

func TestHandleJSONRPC_InitializeCapabilitiesJSON(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)

	// Marshal and verify the JSON output includes "tools" in capabilities
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	result, ok := raw["result"].(map[string]interface{})
	require.True(t, ok, "result should be a JSON object")

	capabilities, ok := result["capabilities"].(map[string]interface{})
	require.True(t, ok, "capabilities should be a JSON object")

	_, hasTools := capabilities["tools"]
	assert.True(t, hasTools, "capabilities must contain 'tools' key for MCP clients to discover tool support")
}

func TestHandleJSONRPC_ToolsList(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(2),
		Method:  "tools/list",
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(ToolListResult)
	require.True(t, ok)
	assert.Equal(t, 8, len(result.Tools))

	// Verify tool names
	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}
	assert.True(t, toolNames["help"])
	assert.True(t, toolNames["identify"])
	assert.True(t, toolNames["describe_model"])
	assert.True(t, toolNames["search"])
	assert.True(t, toolNames["get_entity"])
	assert.True(t, toolNames["list_entities"])
	assert.True(t, toolNames["validate"])
	assert.True(t, toolNames["generate_document"])
}

func TestHandleJSONRPC_ToolsCall(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(3),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "help",
			"arguments": map[string]interface{}{},
		},
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestHandleJSONRPC_Ping(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(4),
		Method:  "ping",
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)
}

func TestHandleJSONRPC_UnknownMethod(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(5),
		Method:  "nonexistent/method",
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
}

func TestHandleJSONRPC_NotificationInitialized(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	resp := HandleJSONRPC(req, ctx)
	assert.Nil(t, resp, "Notifications should not produce a response")
}

func TestHandleJSONRPC_ToolsCallUnknownTool(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(6),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "nonexistent_tool",
			"arguments": map[string]interface{}{},
		},
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error, "Unknown tool should be a tool-level error, not RPC error")

	result, ok := resp.Result.(*ToolCallResult)
	require.True(t, ok)
	assert.True(t, result.IsError)
}

func TestHandleJSONRPC_ToolsCallMissingName(t *testing.T) {
	ctx := newTestToolContext()
	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(7),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"arguments": map[string]interface{}{},
		},
	}

	resp := HandleJSONRPC(req, ctx)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, -32602, resp.Error.Code)
}
