// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"

	"gopkg.in/yaml.v3"
)

// uapfManifestFiles are the candidate UAPF package manifest filenames in the
// repository root, in priority order.
var uapfManifestFiles = []string{
	"uapf.yaml", "uapf.yml",
	"process.uapf.yaml", "process.uapf.yml",
}

const maxUAPFManifestSize int64 = 256 * 1024 // 256 KB

// uapfManifest is the subset of a UAPF package manifest the UAPF-IP descriptor
// needs. Unknown fields are ignored.
type uapfManifest struct {
	Kind                 string   `yaml:"kind"`
	ID                   string   `yaml:"id"`
	Name                 string   `yaml:"name"`
	Description          string   `yaml:"description"`
	Version              string   `yaml:"version"`
	Level                int      `yaml:"level"`
	RequiresCapabilities []string `yaml:"requires_capabilities"`
	ProfilesSupported    []string `yaml:"profiles_supported"`
	Guardrails           string   `yaml:"guardrails"`
}

// UAPFIPEndpoint serves the UAPF-IP package descriptor for a repository.
//
// This is the UAPF-IP (UAPF Integration Protocol) counterpart to the per-repo
// MCP endpoint. Where /mcp exposes a repo's content for AI agents to read,
// /uapf-ip exposes the repo AS AN EXECUTABLE PACKAGE: a conforming runtime can
// discover the package id and version, its declared capability needs, its
// guardrails reference and its supported profiles — plus a link to fetch the
// package archive — without cloning and unpacking the repository first.
//
// ProcessGit is the registry / distribution side of UAPF-IP. The host and
// runtime protocol endpoints belong to hosts and runtimes respectively, not to
// a package source; ProcessGit therefore serves a descriptor, not a session
// surface.
func UAPFIPEndpoint(ctx *context.Context) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]string{"error": "repository is empty"})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	var (
		manifest     *uapfManifest
		manifestName string
	)
	for _, name := range uapfManifestFiles {
		entry, err := commit.GetTreeEntryByPath(name)
		if err != nil {
			if git.IsErrNotExist(err) {
				continue
			}
			ctx.ServerError("GetTreeEntryByPath", err)
			return
		}
		if entry.IsDir() || entry.Blob().Size() > maxUAPFManifestSize {
			continue
		}
		reader, err := entry.Blob().DataAsync()
		if err != nil {
			ctx.ServerError("Blob.DataAsync", err)
			return
		}
		var m uapfManifest
		decErr := yaml.NewDecoder(reader).Decode(&m)
		_ = reader.Close()
		if decErr != nil {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "invalid UAPF manifest " + name + ": " + decErr.Error(),
			})
			return
		}
		manifest = &m
		manifestName = name
		break
	}

	if manifest == nil {
		ctx.JSON(http.StatusNotFound, map[string]string{
			"error": "not a UAPF package repository (no uapf.yaml / process.uapf.yaml in root)",
		})
		return
	}

	base := strings.TrimSuffix(setting.AppURL, "/")
	repoPath := ctx.Repo.Repository.FullName()
	branch := ctx.Repo.Repository.DefaultBranch
	rawBase := base + "/" + repoPath + "/raw/branch/" + branch

	guardrails := map[string]any{"declared": false}
	if manifest.Guardrails != "" {
		guardrails = map[string]any{
			"declared": true,
			"path":     manifest.Guardrails,
			"url":      rawBase + "/" + manifest.Guardrails,
		}
	}

	ctx.JSON(http.StatusOK, map[string]any{
		"uapfIp":     "v0.1",
		"role":       "package-registry",
		"repository": repoPath,
		"ref":        branch,
		"package": map[string]any{
			"id":      manifest.ID,
			"name":    manifest.Name,
			"version": manifest.Version,
			"level":   manifest.Level,
			"kind":    manifest.Kind,
		},
		"requires_capabilities": manifest.RequiresCapabilities,
		"profiles_supported":    manifest.ProfilesSupported,
		"guardrails":            guardrails,
		"distribution": map[string]any{
			"manifest": manifestName,
			"archive":  base + "/" + repoPath + "/archive/" + branch + ".zip",
			"raw":      rawBase,
		},
	})
}
