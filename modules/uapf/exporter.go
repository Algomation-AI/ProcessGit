// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
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

	requiredPaths := make(map[string]struct{}, len(refPaths))
	for _, rel := range refPaths {
		if rel == "" {
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
		requiredPaths[rel] = struct{}{}
	}

	entries, err := commit.Tree.ListEntriesRecursiveFast()
	if err != nil {
		return nil, "", err
	}

	pr, pw := io.Pipe()
	go func() {
		zw := zip.NewWriter(pw)
		if err := writeBytesEntry(zw, "manifest.json", manifestData); err != nil {
			_ = pw.CloseWithError(err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "" || name == "manifest.json" {
				delete(requiredPaths, name)
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
			delete(requiredPaths, name)
		}

		if len(requiredPaths) > 0 {
			missing := make([]string, 0, len(requiredPaths))
			for path := range requiredPaths {
				missing = append(missing, path)
			}
			slices.Sort(missing)
			_ = pw.CloseWithError(fmt.Errorf("referenced path missing at ref %s: %s", ref, strings.Join(missing, ", ")))
			return
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
