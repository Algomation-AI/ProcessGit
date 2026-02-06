// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/json"
)

// ToolContext holds everything a tool needs to execute.
type ToolContext struct {
	Config *MCPConfig
	Commit *git.Commit
	RepoID int64
	Index  *EntityIndex
}

// ToolHandler is a function that executes a tool and returns a result.
type ToolHandler func(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error)

// toolRegistry maps tool names to handlers.
// Populated in init() to avoid circular initialization with tool functions
// that reference toolRegistry (e.g. toolIdentify uses len(toolRegistry)).
var toolRegistry map[string]ToolHandler

func init() {
	toolRegistry = map[string]ToolHandler{
		"help":              toolHelp,
		"identify":          toolIdentify,
		"describe_model":    toolDescribeModel,
		"search":            toolSearch,
		"get_entity":        toolGetEntity,
		"list_entities":     toolListEntities,
		"validate":          toolValidate,
		"generate_document": toolGenerateDocument,
	}
}

// GetToolDefinitions returns the MCP tool definitions for tools/list.
func GetToolDefinitions(cfg *MCPConfig) []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "help",
			Description: "Describes what this MCP server does, what tools are available, and how to use them. Call this first to understand the server's capabilities.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "identify",
			Description: "Returns server identity: name, version, repository info, and operator metadata.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name: "describe_model",
			Description: "Returns the data model: entity types, their attributes, hierarchy, and counts. " +
				"Use this to understand what data is available before searching or listing.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name: "search",
			Description: fmt.Sprintf(
				"Full-text search across all entities in '%s'. Searches by name, code, registration number (NMR), "+
					"document prefix, or any attribute value. Returns matching entities with full details.",
				cfg.Server.Name,
			),
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"query"},
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query — entity name, code number, registration number (NMR), or any attribute value",
					},
					"limit": map[string]interface{}{
						"type":        "number",
						"description": "Maximum results to return (default 25, max 100)",
					},
				},
			},
		},
		{
			Name:        "get_entity",
			Description: "Retrieve full details of a specific entity by its ID. Entity IDs are formatted as 'type:code', e.g., 'ministry:01', 'organization:0001'. Use list_entities or search to discover IDs.",
			InputSchema: map[string]interface{}{
				"type":     "object",
				"required": []string{"id"},
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Entity ID in 'type:code' format, e.g., 'ministry:01' or 'organization:0001'",
					},
				},
			},
		},
		{
			Name: "list_entities",
			Description: "List all entities, optionally filtered by type and/or parent. " +
				"Useful for getting all ministries, or all organizations under a specific ministry.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by entity type, e.g., 'ministry' or 'organization'",
					},
					"parent": map[string]interface{}{
						"type":        "string",
						"description": "Filter by parent entity ID, e.g., 'ministry:13' to list only organizations under that ministry",
					},
				},
			},
		},
		{
			Name: "validate",
			Description: "Validate the XML data source against its schema. Returns validation status, " +
				"any errors found, and data statistics (entity counts).",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name: "generate_document",
			Description: "Generate a formatted Markdown document (table) of the register contents. " +
				"Produces a human-readable view of the full data, organized by hierarchy. " +
				"Optionally filter by type or parent to generate partial documents.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Generate document only for this entity type",
					},
					"parent": map[string]interface{}{
						"type":        "string",
						"description": "Generate document only for children of this parent entity",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Output format: 'markdown' (default) or 'csv'",
						"enum":        []string{"markdown", "csv"},
					},
				},
			},
		},
	}
}

// ExecuteTool runs a named tool with the given arguments.
func ExecuteTool(ctx *ToolContext, name string, args map[string]interface{}) (*ToolCallResult, error) {
	handler, ok := toolRegistry[name]
	if !ok {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", name)}},
			IsError: true,
		}, nil
	}
	return handler(ctx, args)
}

// textResult is a helper to return a simple text result.
func textResult(text string) *ToolCallResult {
	return &ToolCallResult{
		Content: []ToolContent{{Type: "text", Text: text}},
	}
}

// jsonTextResult marshals data to JSON and returns it as text content.
func jsonTextResult(data interface{}) (*ToolCallResult, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return textResult(string(jsonBytes)), nil
}
