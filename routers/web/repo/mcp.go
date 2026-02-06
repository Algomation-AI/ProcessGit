// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/mcp"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

// MCPEndpoint handles MCP JSON-RPC requests for a repository.
func MCPEndpoint(ctx *context.Context) {
	if !setting.MCP.Enabled {
		ctx.JSON(http.StatusNotFound, map[string]string{"error": "MCP is disabled on this instance"})
		return
	}

	// Get the default branch commit
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]string{"error": "repository is empty"})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	// Load MCP config
	cfg, err := mcp.LoadConfig(commit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to load MCP config: " + err.Error(),
		})
		return
	}
	if cfg == nil {
		ctx.JSON(http.StatusNotFound, map[string]string{
			"error": "MCP not enabled for this repository (no processgit.mcp.yaml found)",
		})
		return
	}

	// Build entity index
	index, err := mcp.GetOrBuildIndex(ctx.Repo.Repository.ID, commit, cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to build index: " + err.Error(),
		})
		return
	}

	// Build tool context
	toolCtx := &mcp.ToolContext{
		Config: cfg,
		Commit: commit,
		RepoID: ctx.Repo.Repository.ID,
		Index:  index,
	}

	// Delegate to MCP transport
	mcp.ServeHTTP(ctx.Resp, ctx.Req, toolCtx)
}
