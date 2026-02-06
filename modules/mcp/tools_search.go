// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import "fmt"

func toolSearch(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: "Error: 'query' parameter is required"}},
			IsError: true,
		}, nil
	}

	limit := 25
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
		if limit > 100 {
			limit = 100
		}
	}

	results := ctx.Index.SearchEntities(query, limit)

	if len(results) == 0 {
		return textResult(fmt.Sprintf("No entities found matching '%s'.", query)), nil
	}

	return jsonTextResult(map[string]interface{}{
		"query":   query,
		"count":   len(results),
		"results": results,
	})
}
