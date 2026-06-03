// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
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

	manifestName, manifestEntry, err := findManifestEntry(commit)
	if err != nil {
		return nil, "", err
	}

	manifestData, err := readTreeEntry(manifestEntry)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", manifestName, err)
	}

	if err := ValidateManifest(manifestName, manifestData); err != nil {
		return nil, "", err
	}

	manifestJSON, err := manifestToJSON(manifestName, manifestData)
	if err != nil {
		return nil, "", err
	}

	var manifest spec.Manifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return nil, "", fmt.Errorf("manifest is not valid: %w", err)
	}

	if err := spec.ValidateManifest(&manifest); err != nil {
		return nil, "", err
	}

	entries, err := commit.Tree.ListEntriesRecursiveFast()
	if err != nil {
		return nil, "", err
	}

	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		if err := writeBytesEntry(zw, manifestName, manifestData); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "" || name == manifestName {
				continue
			}
			if entry.IsSubModule() {
				_ = pw.CloseWithError(fmt.Errorf("exporting submodules is not supported: %s", name))
				return
			}
			if err := writeTreeEntry(zw, entry, name); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}

		if err := zw.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	filename := buildExportFilename(repo, manifest)
	return pr, filename, nil
}

// findManifestEntry locates the UAPF manifest in a commit tree, returning the
// highest-priority accepted manifest file name and its tree entry.
func findManifestEntry(commit *git.Commit) (string, *git.TreeEntry, error) {
	for _, name := range ManifestNames {
		entry, err := commit.GetTreeEntryByPath(name)
		if err == nil {
			return name, entry, nil
		}
		if !git.IsErrNotExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("a UAPF manifest (uapf.yaml) is required in the repository")
}

func buildExportFilename(repo *repo_model.Repository, manifest spec.Manifest) string {
	name := manifest.Name
	version := manifest.Version
	if name == "" {
		name = manifest.ID
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

func writeBytesEntry(zw *zip.Writer, name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(0o644)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(data)
	return err
}

func writeTreeEntry(zw *zip.Writer, entry *git.TreeEntry, name string) error {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return err
	}
	defer reader.Close()

	mode := os.FileMode(0o644)
	if entry.IsExecutable() {
		mode = 0o755
	}
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, reader)
	return err
}

func readTreeEntry(entry *git.TreeEntry) ([]byte, error) {
	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
