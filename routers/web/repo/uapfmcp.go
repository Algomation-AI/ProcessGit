// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	stdctx "context"
	"net/http"
	"os"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"

	uapfengine "github.com/UAPFormat/uapf-mcp-go/engine"
	uapfmcp "github.com/UAPFormat/uapf-mcp-go/mcp"
	"github.com/UAPFormat/uapf-mcp-go/pkgsource"

	"gopkg.in/yaml.v3"
)

// UAPFMCPEndpoint serves the per-repo UAPF *execution* MCP scope.
//
// Where /mcp exposes the repo's content as read/reason tools and /uapf-ip
// serves a static package descriptor, /uapf-mcp exposes the repo as a RUNNABLE
// package: an agent (e.g. Copilot Studio) connects here and gets execution
// tools (start_session, execute_process, evaluate_decision, get_audit, ...)
// that ProcessGit delegates to the UAPF engine. ProcessGit terminates the MCP
// protocol and forwards execution to the engine; it does not itself hold
// sessions or evaluate decisions — the runtime is the engine.
//
// The package is resolved live from the repo's uapf.yaml manifest on each
// connection, so a push updates the executable surface with no redeploy.
func UAPFMCPEndpoint(ctx *context.Context) {
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]string{"error": "repository is empty"})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	manifest, _, wrote := readUAPFManifest(ctx, commit)
	if wrote {
		return
	}
	if manifest == nil {
		ctx.JSON(http.StatusNotFound, map[string]string{
			"error": "not a UAPF package repository (no uapf.yaml / process.uapf.yaml in root)",
		})
		return
	}

	repoPath := ctx.Repo.Repository.FullName()
	branch := ctx.Repo.Repository.DefaultBranch
	archiveBase := strings.TrimSuffix(envOr("UAPF_ARCHIVE_BASE", strings.TrimSuffix(setting.AppURL, "/")), "/")

	pkg := &pkgsource.Package{
		Ref:                  repoPath,
		ID:                   manifest.ID,
		Name:                 manifest.Name,
		Version:              manifest.Version,
		Level:                manifest.Level,
		Kind:                 manifest.Kind,
		ArchiveURL:           archiveBase + "/" + repoPath + "/archive/" + branch + ".zip",
		RequiresCapabilities: manifest.RequiresCapabilities,
		Guardrails:           manifest.Guardrails,
	}

	eng, host := uapfRuntime()
	srv := uapfmcp.NewServer(staticUAPFProvider{pkg: pkg}, eng)
	srv.Host = host
	srv.HandleMCP(ctx.Resp, ctx.Req, repoPath)
}

// readUAPFManifest reads the repo's UAPF manifest from the given commit.
// Returns (manifest, filename, responseWritten). On a malformed manifest it
// writes a 422 and returns responseWritten=true. A nil manifest with
// responseWritten=false means no manifest is present in the repo root.
func readUAPFManifest(ctx *context.Context, commit *git.Commit) (*uapfManifest, string, bool) {
	for _, name := range uapfManifestFiles {
		entry, err := commit.GetTreeEntryByPath(name)
		if err != nil {
			if git.IsErrNotExist(err) {
				continue
			}
			ctx.ServerError("GetTreeEntryByPath", err)
			return nil, "", true
		}
		if entry.IsDir() || entry.Blob().Size() > maxUAPFManifestSize {
			continue
		}
		reader, err := entry.Blob().DataAsync()
		if err != nil {
			ctx.ServerError("Blob.DataAsync", err)
			return nil, "", true
		}
		var m uapfManifest
		decErr := yaml.NewDecoder(reader).Decode(&m)
		_ = reader.Close()
		if decErr != nil {
			ctx.JSON(http.StatusUnprocessableEntity, map[string]string{
				"error": "invalid UAPF manifest " + name + ": " + decErr.Error(),
			})
			return nil, "", true
		}
		return &m, name, false
	}
	return nil, "", false
}

// staticUAPFProvider implements pkgsource.Provider for a single, pre-resolved
// package (read live from the repo's git tree in the handler).
type staticUAPFProvider struct{ pkg *pkgsource.Package }

func (p staticUAPFProvider) Resolve(_ stdctx.Context, _ string) (*pkgsource.Package, error) {
	return p.pkg, nil
}

var (
	uapfRuntimeOnce sync.Once
	uapfEngineCli   *uapfengine.Client
	uapfHostCfg     uapfmcp.HostConfig
)

// uapfRuntime lazily builds the shared engine client + capability-host config
// from environment (wired to the bundled engine + LLM gateway in the release).
func uapfRuntime() (*uapfengine.Client, uapfmcp.HostConfig) {
	uapfRuntimeOnce.Do(func() {
		uapfEngineCli = uapfengine.New(envOr("UAPF_ENGINE_URL", "http://uapf-engine:4000"))
		uapfHostCfg = uapfmcp.HostConfig{
			HostDID:     envOr("UAPF_HOST_DID", "did:web:processgit.local"),
			HostBaseURL: os.Getenv("UAPF_HOST_BASE_URL"),
			Profiles:    []string{"uapf-ip-orchestrated"},
		}
	})
	return uapfEngineCli, uapfHostCfg
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
