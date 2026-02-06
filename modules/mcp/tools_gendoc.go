// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"sort"
	"strings"
)

func toolGenerateDocument(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	typeFilter, _ := args["type"].(string)
	parentFilter, _ := args["parent"].(string)
	format, _ := args["format"].(string)
	if format == "" {
		format = "markdown"
	}

	switch format {
	case "markdown":
		return generateMarkdown(ctx, typeFilter, parentFilter)
	case "csv":
		return generateCSV(ctx, typeFilter, parentFilter)
	default:
		return textResult(fmt.Sprintf("Unknown format '%s'. Use 'markdown' or 'csv'.", format)), nil
	}
}

func generateMarkdown(ctx *ToolContext, typeFilter, parentFilter string) (*ToolCallResult, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", ctx.Config.Server.Name))
	if ctx.Config.Server.Description != "" {
		sb.WriteString(ctx.Config.Server.Description + "\n\n")
	}

	commitPrefix := ctx.Index.CommitSHA
	if len(commitPrefix) > 8 {
		commitPrefix = commitPrefix[:8]
	}
	sb.WriteString(fmt.Sprintf("*Source: %s | Commit: %s*\n\n", ctx.Index.SourceFile, commitPrefix))

	// Determine what entity types to show (find the "top-level" types)
	topTypes := findTopLevelTypes(ctx.Index)

	for _, topType := range topTypes {
		if typeFilter != "" && typeFilter != topType {
			continue
		}

		topIDs := ctx.Index.ByType[topType]
		sortedIDs := make([]string, len(topIDs))
		copy(sortedIDs, topIDs)
		sort.Strings(sortedIDs)

		for _, topID := range sortedIDs {
			if parentFilter != "" && topID != parentFilter {
				continue
			}

			topEntity := ctx.Index.Entities[topID]
			if topEntity == nil {
				continue
			}

			// Section header for top-level entity
			headerName := topEntity.Name
			if headerName == "" {
				headerName = topEntity.ID
			}
			sb.WriteString(fmt.Sprintf("## %s (code: %s)\n\n",
				headerName, topEntity.Attributes["code"]))

			// Children as table
			childIDs, hasChildren := ctx.Index.ByParent[topID]
			if hasChildren && len(childIDs) > 0 {
				// Collect all attribute keys from children
				attrKeys := collectChildAttributeKeys(ctx.Index, childIDs)

				// Table header
				sb.WriteString("| # | Name |")
				for _, key := range attrKeys {
					sb.WriteString(fmt.Sprintf(" %s |", key))
				}
				sb.WriteString("\n|---|------|")
				for range attrKeys {
					sb.WriteString("------|")
				}
				sb.WriteString("\n")

				// Table rows
				sortedChildIDs := make([]string, len(childIDs))
				copy(sortedChildIDs, childIDs)
				sort.Strings(sortedChildIDs)

				for i, childID := range sortedChildIDs {
					child := ctx.Index.Entities[childID]
					if child == nil {
						continue
					}
					sb.WriteString(fmt.Sprintf("| %d | %s |", i+1, child.Name))
					for _, key := range attrKeys {
						val := child.Attributes[key]
						sb.WriteString(fmt.Sprintf(" %s |", val))
					}
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}
		}
	}

	// Summary
	sb.WriteString("---\n\n")
	sb.WriteString("## Summary\n\n")
	for typeName, count := range ctx.Index.Stats.TypeCounts {
		sb.WriteString(fmt.Sprintf("- **%s**: %d\n", typeName, count))
	}
	sb.WriteString(fmt.Sprintf("- **Total entities**: %d\n", ctx.Index.Stats.TotalEntities))

	return textResult(sb.String()), nil
}

func generateCSV(ctx *ToolContext, typeFilter, parentFilter string) (*ToolCallResult, error) {
	var sb strings.Builder

	// CSV header
	sb.WriteString("type,id,name,parent_id,code,nmr,docPrefix\n")

	for _, entity := range ctx.Index.Entities {
		if typeFilter != "" && entity.Type != typeFilter {
			continue
		}
		if parentFilter != "" && entity.ParentID != parentFilter {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s,%s,\"%s\",%s,%s,%s,%s\n",
			entity.Type,
			entity.ID,
			strings.ReplaceAll(entity.Name, "\"", "\"\""),
			entity.ParentID,
			entity.Attributes["code"],
			entity.Attributes["nmr"],
			entity.Attributes["docPrefix"],
		))
	}

	return textResult(sb.String()), nil
}

// findTopLevelTypes returns entity types that have no parent (root types).
func findTopLevelTypes(index *EntityIndex) []string {
	var topTypes []string
	for typeName, ids := range index.ByType {
		isTop := false
		for _, id := range ids {
			if entity, ok := index.Entities[id]; ok && entity.ParentID == "" {
				isTop = true
				break
			}
		}
		if isTop {
			topTypes = append(topTypes, typeName)
		}
	}
	sort.Strings(topTypes)
	return topTypes
}

// collectChildAttributeKeys returns sorted unique attribute keys from child entities.
func collectChildAttributeKeys(index *EntityIndex, childIDs []string) []string {
	keySet := make(map[string]bool)
	for _, id := range childIDs {
		if e, ok := index.Entities[id]; ok {
			for k := range e.Attributes {
				keySet[k] = true
			}
		}
	}
	var keys []string
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
