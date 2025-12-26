// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/uapf"
	files_service "code.gitea.io/gitea/services/repository/files"
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

	destinationPath, err := buildUAPFTreePath(ctx.Req.FormValue("dest"), filename)
	if err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	if err := uapf.ValidatePackage(buffer); err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	defaultBranch := ctx.Repo.Repository.DefaultBranch
	changeOpts := &files_service.ChangeRepoFilesOptions{
		OldBranch: defaultBranch,
		NewBranch: defaultBranch,
		Message:   fmt.Sprintf("Import UAPF package: %s", filename),
		Files: []*files_service.ChangeRepoFile{
			{
				Operation:     "create",
				TreePath:      destinationPath,
				ContentReader: bytes.NewReader(buffer),
			},
		},
		Author: &files_service.IdentityOptions{
			GitUserName:  ctx.Doer.GitName(),
			GitUserEmail: ctx.Doer.GetEmail(),
		},
		Committer: &files_service.IdentityOptions{
			GitUserName:  ctx.Doer.GitName(),
			GitUserEmail: ctx.Doer.GetEmail(),
		},
	}

	if _, err := files_service.ChangeRepoFiles(ctx, ctx.Repo.Repository, ctx.Doer, changeOpts); err != nil {
		ctx.Flash.Error(err.Error())
		ctx.Redirect(ctx.Repo.RepoLink)
		return
	}

	ctx.Flash.Success(fmt.Sprintf("Imported %s into %s", filename, destinationPath))
	ctx.Redirect(ctx.Repo.RepoLink)
}

func buildUAPFTreePath(destination, filename string) (string, error) {
	filename = path.Base(filename)
	if filename == "." || filename == "" {
		return "", fmt.Errorf("invalid filename for UAPF package")
	}

	destination = strings.TrimSpace(destination)
	if destination == "" {
		destination = "uapf"
	}

	cleanDestination := path.Clean("/" + destination)
	cleanDestination = strings.TrimPrefix(cleanDestination, "/")
	if cleanDestination == "." {
		cleanDestination = ""
	}
	if strings.HasPrefix(cleanDestination, "..") {
		return "", fmt.Errorf("invalid destination path")
	}

	if cleanDestination == "" {
		return filename, nil
	}
	return path.Join(cleanDestination, filename), nil
}
