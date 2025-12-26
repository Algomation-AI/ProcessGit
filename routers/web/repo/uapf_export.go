// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"io"

	"code.gitea.io/gitea/modules/uapf"
	"code.gitea.io/gitea/services/context"
)

// UAPFExportGet streams a .uapf package for the repository contents.
func UAPFExportGet(ctx *context.Context) {
	ref := ctx.FormString("ref")

	reader, filename, err := uapf.ExportUAPF(ctx, ctx.Repo.Repository, ref)
	if err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}
	defer reader.Close()

	ctx.Resp.Header().Set("Content-Type", "application/zip")
	ctx.Resp.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	_, _ = io.Copy(ctx.Resp, reader)
}
