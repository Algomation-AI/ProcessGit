// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"io"
	"net/http"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// ProcessGitViewerContent returns repository file content for ProcessGit viewers.
func ProcessGitViewerContent(ctx *context.Context) {
	treePath := strings.TrimSpace(ctx.FormString("path"))
	if treePath == "" {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "path is required"})
		return
	}

	cleanPath := util.PathJoinRel(treePath)
	if cleanPath == "" || cleanPath == "." || cleanPath == "/" || strings.HasPrefix(cleanPath, "../") {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	ref := strings.TrimSpace(ctx.FormString("ref"))
	if ref == "" {
		ref = ctx.Repo.CommitID
	}
	if ref == "" {
		ref = ctx.Repo.Repository.DefaultBranch
	}

	commit, err := ctx.Repo.GitRepo.GetCommit(ref)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetCommit", err)
		}
		return
	}

	entry, err := commit.GetTreeEntryByPath(cleanPath)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetTreeEntryByPath", err)
		}
		return
	}
	if entry.IsDir() {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "path points to a directory"})
		return
	}

	blob := entry.Blob()

	prefetchBuf, dataRc, fInfo, err := getFileReader(ctx, ctx.Repo.Repository.ID, blob)
	if err != nil {
		ctx.ServerError("getFileReader", err)
		return
	}
	defer dataRc.Close()

	if fInfo.blobOrLfsSize >= setting.UI.MaxDisplayFileSize {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "file is too large to render"})
		return
	}

	reader := io.MultiReader(bytes.NewReader(prefetchBuf), dataRc)
	if fInfo.st.IsRepresentableAsText() {
		reader = charset.ToUTF8WithFallbackReader(reader, charset.ConvertOpts{})
	}

	content, err := io.ReadAll(io.LimitReader(reader, setting.UI.MaxDisplayFileSize))
	if err != nil {
		ctx.ServerError("ReadAll", err)
		return
	}

	ctx.JSON(http.StatusOK, map[string]string{
		"content": string(content),
		"path":    path.Clean(cleanPath),
		"ref":     ref,
	})
}
