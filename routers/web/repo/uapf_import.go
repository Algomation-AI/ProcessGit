// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/uapf"
	"code.gitea.io/gitea/services/context"
)

// UAPFImportPost handles importing a .uapf package into a repository.
func UAPFImportPost(ctx *context.Context) {
	upload, header, err := ctx.Req.FormFile("uapf")
	if err != nil {
		ctx.Flash.Error("Could not read the uploaded UAPF package.")
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}
	defer upload.Close()

	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".uapf") {
		ctx.Flash.Error("Only .uapf files can be imported.")
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	buffer, err := io.ReadAll(upload)
	if err != nil {
		ctx.ServerError("ReadAll", err)
		return
	}

	if err := uapf.ImportUAPF(ctx, ctx.Repo.Repository, ctx.Doer, fmt.Sprintf("Import UAPF package: %s", filename), bytes.NewReader(buffer), int64(len(buffer)), "/"); err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	ctx.Flash.Success(fmt.Sprintf("Imported %s into repository root", filename))
	ctx.Redirect(ctx.Repo.RepoLink)
}
