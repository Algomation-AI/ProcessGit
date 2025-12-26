// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/uapf/spec"
)

// ExportUAPF builds a .uapf archive from repository contents at the given ref.
func ExportUAPF(ctx context.Context, repo *repo_model.Repository, ref string) (io.ReadCloser, string, error) {
	gr, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
	if err != nil {
		return nil, "", err
	}
	defer closer.Close()

	if ref == "" {
		ref = repo.DefaultBranch
	}

	commit, err := gr.GetCommit(ref)
	if err != nil {
		return nil, "", err
	}

	manifestEntry, err := commit.GetTreeEntryByPath("manifest.json")
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, "", fmt.Errorf("manifest.json not found at ref %s", ref)
		}
		return nil, "", err
	}

	manifestData, err := readTreeEntry(manifestEntry)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest.json: %w", err)
	}

	if err := ValidateManifest(manifestData); err != nil {
		return nil, "", err
	}

	var manifest spec.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, "", fmt.Errorf("manifest.json is not valid JSON: %w", err)
	}

	refPaths, err := spec.ValidateManifest(&manifest)
	if err != nil {
		return nil, "", err
	}

	type fileEntry struct {
		Path string
		Data []byte
	}
	files := []fileEntry{
		{Path: "manifest.json", Data: manifestData},
	}
	seen := map[string]struct{}{
		"manifest.json": {},
	}

	for _, rel := range refPaths {
		if rel == "" {
			continue
		}
		if _, exists := seen[rel]; exists {
			continue
		}
		entry, err := commit.GetTreeEntryByPath(rel)
		if err != nil {
			if git.IsErrNotExist(err) {
				return nil, "", fmt.Errorf("referenced path missing at ref %s: %s", ref, rel)
			}
			return nil, "", err
		}
		if entry.IsDir() {
			return nil, "", fmt.Errorf("referenced path must be a file: %s", rel)
		}
		data, err := readTreeEntry(entry)
		if err != nil {
			return nil, "", fmt.Errorf("read %s: %w", rel, err)
		}
		files = append(files, fileEntry{Path: rel, Data: data})
		seen[rel] = struct{}{}
	}

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		zw := zip.NewWriter(pw)
		for _, file := range files {
			writer, err := zw.Create(file.Path)
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			if _, err := writer.Write(file.Data); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		_ = zw.Close()
	}()

	filename := buildExportFilename(repo, manifest)
	return pr, filename, nil
}

func buildExportFilename(repo *repo_model.Repository, manifest spec.Manifest) string {
	name := manifest.Name
	version := manifest.Version
	if manifest.Package != nil {
		if manifest.Package.Name != "" {
			name = manifest.Package.Name
		}
		if manifest.Package.Version != "" {
			version = manifest.Package.Version
		}
	}
	if name == "" {
		name = repo.Name
	}
	if version == "" {
		return sanitizeFilename(name) + ".uapf"
	}
	return fmt.Sprintf("%s_%s.uapf", sanitizeFilename(name), sanitizeFilename(version))
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func readTreeEntry(entry *git.TreeEntry) ([]byte, error) {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
