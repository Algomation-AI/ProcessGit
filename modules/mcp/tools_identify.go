// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

func toolIdentify(ctx *ToolContext, args map[string]interface{}) (*ToolCallResult, error) {
	result := map[string]interface{}{
		"server": map[string]interface{}{
			"name":        ctx.Config.Server.Name,
			"version":     "1.0",
			"protocol":    "MCP 2025-03-26",
			"transport":   "Streamable HTTP",
			"tools_count": len(toolRegistry),
			"read_only":   true,
		},
		"repository": map[string]interface{}{
			"commit": ctx.Commit.ID.String(),
		},
		"platform": map[string]interface{}{
			"name":    "ProcessGit",
			"version": "1.0",
		},
		"sources": ctx.Config.Sources,
	}
	return jsonTextResult(result)
}
