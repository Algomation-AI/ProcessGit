// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import "fmt"

func toolListEntities(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	typeFilter, _ := args["type"].(string)
	parentFilter, _ := args["parent"].(string)

	var results []*Entity

	if parentFilter != "" {
		// List children of a specific parent
		childIDs, ok := ctx.Index.ByParent[parentFilter]
		if !ok {
			return textResult(fmt.Sprintf("No children found for parent '%s'.", parentFilter)), nil
		}
		for _, id := range childIDs {
			if entity, ok := ctx.Index.Entities[id]; ok {
				if typeFilter == "" || entity.Type == typeFilter {
					results = append(results, entity)
				}
			}
		}
	} else if typeFilter != "" {
		// List all entities of a type
		ids, ok := ctx.Index.ByType[typeFilter]
		if !ok {
			// List available types
			var types []string
			for t := range ctx.Index.ByType {
				types = append(types, t)
			}
			return textResult(fmt.Sprintf("Unknown type '%s'. Available types: %v", typeFilter, types)), nil
		}
		for _, id := range ids {
			if entity, ok := ctx.Index.Entities[id]; ok {
				results = append(results, entity)
			}
		}
	} else {
		// List all entities
		for _, entity := range ctx.Index.Entities {
			results = append(results, entity)
		}
	}

	return jsonTextResult(map[string]interface{}{
		"count":    len(results),
		"filters":  map[string]interface{}{"type": typeFilter, "parent": parentFilter},
		"entities": results,
	})
}
