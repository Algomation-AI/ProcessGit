// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import "fmt"

func toolGetEntity(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	id, _ := args["id"].(string)
	if id == "" {
		return &ToolCallResult{
			Content: []ToolContent{{Type: "text", Text: "Error: 'id' parameter is required. Use format 'type:code', e.g., 'ministry:01'."}},
			IsError: true,
		}, nil
	}

	entity, ok := ctx.Index.Entities[id]
	if !ok {
		// Try to be helpful — suggest similar IDs
		suggestions := ctx.Index.SearchEntities(id, 3)
		msg := fmt.Sprintf("Entity '%s' not found.", id)
		if len(suggestions) > 0 {
			msg += " Did you mean: "
			for i, s := range suggestions {
				if i > 0 {
					msg += ", "
				}
				msg += fmt.Sprintf("'%s' (%s)", s.ID, s.Name)
			}
			msg += "?"
		}
		return textResult(msg), nil
	}

	// Build rich response with children
	response := map[string]interface{}{
		"id":         entity.ID,
		"type":       entity.Type,
		"name":       entity.Name,
		"attributes": entity.Attributes,
	}

	if entity.ParentID != "" {
		response["parent_id"] = entity.ParentID
		if parent, ok := ctx.Index.Entities[entity.ParentID]; ok {
			response["parent_name"] = parent.Name
		}
	}

	// Include children with details
	if childIDs, ok := ctx.Index.ByParent[id]; ok && len(childIDs) > 0 {
		var children []map[string]interface{}
		for _, childID := range childIDs {
			if child, ok := ctx.Index.Entities[childID]; ok {
				children = append(children, map[string]interface{}{
					"id":         child.ID,
					"name":       child.Name,
					"attributes": child.Attributes,
				})
			}
		}
		response["children"] = children
		response["children_count"] = len(children)
	}

	return jsonTextResult(response)
}
