// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

// MCPConfig represents the parsed processgit.mcp.yaml file.
type MCPConfig struct {
	Version int             `yaml:"version"`
	Server  MCPServerConfig `yaml:"server"`
	Sources []MCPSource     `yaml:"sources"`
}

// MCPServerConfig holds server metadata from the config file.
type MCPServerConfig struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Instructions string `yaml:"instructions"`
}

// MCPSource declares a data source file in the repository.
type MCPSource struct {
	Path        string `yaml:"path"`
	Type        string `yaml:"type"`        // "xml", "json", etc.
	Schema      string `yaml:"schema"`      // optional XSD/JSON Schema path
	Description string `yaml:"description"`
}

// --- JSON-RPC 2.0 types ---

// JSONRPCRequest represents an incoming JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// --- MCP protocol types ---

// InitializeParams is sent by the client during the initialize handshake.
type InitializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    struct{}   `json:"capabilities"`
	ClientInfo      ClientInfo `json:"clientInfo"`
}

// ClientInfo identifies the MCP client.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is returned by the server during the initialize handshake.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Instructions    string             `json:"instructions,omitempty"`
}

// ServerCapabilities declares what the server supports.
type ServerCapabilities struct {
	Tools *ToolCapability `json:"tools,omitempty"`
}

// ToolCapability declares tool support.
type ToolCapability struct{}

// ServerInfo identifies the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// --- Tool types ---

// ToolDefinition describes an available tool for tools/list.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// ToolListResult is returned for tools/list requests.
type ToolListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolCallParams is sent by the client when calling a tool.
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult is returned from a tool execution.
type ToolCallResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents a content block in a tool result.
type ToolContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// --- Entity types (parsed from XML) ---

// Entity represents a single parsed entity from the data source.
type Entity struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	ParentID   string            `json:"parent_id,omitempty"`
	Attributes map[string]string `json:"attributes"`
	Children   []string          `json:"children,omitempty"`
}

// EntityIndex holds all parsed entities with lookup indices.
type EntityIndex struct {
	Entities   map[string]*Entity  // keyed by ID
	ByType     map[string][]string // type -> list of IDs
	ByParent   map[string][]string // parentID -> list of child IDs
	SourceFile string
	CommitSHA  string
	Stats      IndexStats
}

// IndexStats holds summary statistics about the index.
type IndexStats struct {
	TotalEntities int
	TypeCounts    map[string]int
}
