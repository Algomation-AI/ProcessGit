// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

func toolDescribeModel(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	// Collect unique attribute names per entity type
	typeAttrs := make(map[string]map[string]bool)
	for _, entity := range ctx.Index.Entities {
		if _, ok := typeAttrs[entity.Type]; !ok {
			typeAttrs[entity.Type] = make(map[string]bool)
		}
		for k := range entity.Attributes {
			typeAttrs[entity.Type][k] = true
		}
	}

	// Build entity type descriptions
	var entityTypes []map[string]interface{}
	for typeName, count := range ctx.Index.Stats.TypeCounts {
		attrs := make([]string, 0)
		if attrSet, ok := typeAttrs[typeName]; ok {
			for attr := range attrSet {
				attrs = append(attrs, attr)
			}
		}

		typeDesc := map[string]interface{}{
			"type":       typeName,
			"count":      count,
			"attributes": attrs,
		}

		// Find if entities of this type have a common parent type
		for _, id := range ctx.Index.ByType[typeName] {
			if e, ok := ctx.Index.Entities[id]; ok && e.ParentID != "" {
				if parent, ok2 := ctx.Index.Entities[e.ParentID]; ok2 {
					typeDesc["parent_type"] = parent.Type
				}
				break
			}
		}

		// Find if entities of this type have children
		for _, id := range ctx.Index.ByType[typeName] {
			if children, ok := ctx.Index.ByParent[id]; ok && len(children) > 0 {
				if child, ok2 := ctx.Index.Entities[children[0]]; ok2 {
					typeDesc["child_type"] = child.Type
				}
				break
			}
		}

		entityTypes = append(entityTypes, typeDesc)
	}

	result := map[string]interface{}{
		"entity_types":   entityTypes,
		"total_entities": ctx.Index.Stats.TotalEntities,
		"source_file":    ctx.Index.SourceFile,
		"commit":         ctx.Index.CommitSHA,
		"id_format":      "type:code (e.g., ministry:01, organization:0001)",
	}

	return jsonTextResult(result)
}
