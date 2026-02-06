// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"

	"code.gitea.io/gitea/modules/json"
)

const (
	// MCPProtocolVersion is the MCP protocol version this server implements.
	MCPProtocolVersion = "2025-03-26"
	// ServerVersion is the version of this MCP server implementation.
	ServerVersion = "0.1.0"
)

// HandleJSONRPC processes a single JSON-RPC request and returns a response.
func HandleJSONRPC(req *JSONRPCRequest, toolCtx *ToolContext) *JSONRPCResponse {
	switch req.Method {

	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: InitializeResult{
				ProtocolVersion: MCPProtocolVersion,
				Capabilities: ServerCapabilities{
					Tools: &ToolCapability{},
				},
				ServerInfo: ServerInfo{
					Name:    toolCtx.Config.Server.Name,
					Version: ServerVersion,
				},
				Instructions: toolCtx.Config.Server.Description,
			},
		}

	case "notifications/initialized":
		// Client acknowledgement — no response needed for notifications
		return nil

	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ToolListResult{
				Tools: GetToolDefinitions(toolCtx.Config),
			},
		}

	case "tools/call":
		return handleToolCall(req, toolCtx)

	case "ping":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		}

	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func handleToolCall(req *JSONRPCRequest, toolCtx *ToolContext) *JSONRPCResponse {
	// Parse ToolCallParams from req.Params
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		return jsonRPCError(req.ID, -32602, "Invalid params")
	}

	var params ToolCallParams
	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		return jsonRPCError(req.ID, -32602, "Invalid tool call params: "+err.Error())
	}

	if params.Name == "" {
		return jsonRPCError(req.ID, -32602, "Missing tool name")
	}

	result, err := ExecuteTool(toolCtx, params.Name, params.Arguments)
	if err != nil {
		return jsonRPCError(req.ID, -32000, "Tool execution error: "+err.Error())
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

func jsonRPCError(id interface{}, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}
