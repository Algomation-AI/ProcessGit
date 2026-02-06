// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import "fmt"

func toolHelp(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	help := fmt.Sprintf(`# %s — MCP Server

%s

## What this server provides

This is a read-only MCP server for a ProcessGit repository. It exposes structured data from the repository as queryable entities, allowing AI agents to search, inspect, and generate documents from the data.

## Available tools

1. **help** — You are here. Describes the server and its tools.
2. **identify** — Server identity and metadata.
3. **describe_model** — Data model overview: what entity types exist, their attributes, hierarchy, and counts. Call this to understand the data structure.
4. **search** — Full-text search across all entities. Search by name, code, registration number, or any attribute. Example: search(query="kanceleja") or search(query="90000038578").
5. **get_entity** — Get full details for one entity by ID. IDs are formatted as "type:code", e.g., "ministry:01" or "organization:0001".
6. **list_entities** — List all entities, filter by type or parent. Example: list_entities(type="ministry") or list_entities(type="organization", parent="ministry:13").
7. **validate** — Check data validity and get statistics.
8. **generate_document** — Generate a formatted Markdown table of the register. Can generate the full register or a filtered subset.

## Recommended workflow

1. Call **describe_model** to understand what data is available
2. Use **search** or **list_entities** to find what you need
3. Use **get_entity** for detailed information about a specific item
4. Use **generate_document** to produce formatted output

## Data sources

This server exposes %d declared source(s):
`, ctx.Config.Server.Name, ctx.Config.Server.Description, len(ctx.Config.Sources))

	for _, src := range ctx.Config.Sources {
		help += fmt.Sprintf("- **%s** (%s)", src.Path, src.Type)
		if src.Description != "" {
			help += " — " + src.Description
		}
		if src.Schema != "" {
			help += fmt.Sprintf(" [schema: %s]", src.Schema)
		}
		help += "\n"
	}

	if ctx.Config.Server.Instructions != "" {
		help += "\n## Additional instructions\n\n" + ctx.Config.Server.Instructions + "\n"
	}

	return textResult(help), nil
}
