// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/uapf/spec"
	files_service "code.gitea.io/gitea/services/repository/files"
)

// ImportUAPF extracts a .uapf archive and commits its contents into the repository.
func ImportUAPF(ctx context.Context, repo *repo_model.Repository, doer *user_model.User, commitMsg string, zipData io.Reader, zipSize int64, targetPath string) error {
	maxSize := setting.Repository.Upload.FileMaxSize << 20
	if maxSize > 0 && zipSize > maxSize {
		return fmt.Errorf("package exceeds maximum size: %d bytes > %d bytes", zipSize, maxSize)
	}

	limitedReader := io.Reader(zipData)
	if maxSize > 0 {
		limitedReader = io.LimitReader(zipData, maxSize+1)
	}

	buffer, err := io.ReadAll(limitedReader)
	if err != nil {
		return fmt.Errorf("read package: %w", err)
	}
	if maxSize > 0 && int64(len(buffer)) > maxSize {
		return fmt.Errorf("package exceeds maximum size: %d bytes > %d bytes", len(buffer), maxSize)
	}

	if err := ValidatePackage(buffer); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "uapf-import-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	readerAt := bytes.NewReader(buffer)
	zipReader, err := zip.NewReader(readerAt, int64(len(buffer)))
	if err != nil {
		return fmt.Errorf("invalid .uapf archive: %w", err)
	}

	if err := extractZipSafe(zipReader, tempDir); err != nil {
		return err
	}

	packageRoot, err := determinePackageRoot(tempDir)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(packageRoot, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest.json is required in the UAPF package")
	}

	if err := ValidateManifest(manifestBytes); err != nil {
		return err
	}

	var manifest spec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("manifest.json is not valid JSON: %w", err)
	}

	refPaths, err := spec.ValidateManifest(&manifest)
	if err != nil {
		return err
	}

	for _, ref := range refPaths {
		if ref == "" {
			return fmt.Errorf("referenced path cannot be empty")
		}
		if _, err := os.Stat(filepath.Join(packageRoot, filepath.FromSlash(ref))); err != nil {
			return fmt.Errorf("referenced path missing in package: %s", ref)
		}
	}

	targetPath, err = normalizeTargetPath(targetPath)
	if err != nil {
		return err
	}

	operations, err := buildFileOperations(ctx, repo, packageRoot, targetPath)
	if err != nil {
		return err
	}

	if commitMsg == "" {
		version := manifest.Version
		name := manifest.Name
		if manifest.Package != nil {
			if manifest.Package.Name != "" {
				name = manifest.Package.Name
			}
			if manifest.Package.Version != "" {
				version = manifest.Package.Version
			}
		}
		if name == "" {
			name = "UAPF package"
		}
		if version != "" {
			commitMsg = fmt.Sprintf("Import UAPF package %s@%s", name, version)
		} else {
			commitMsg = fmt.Sprintf("Import UAPF package %s", name)
		}
	}

	defaultBranch := repo.DefaultBranch
	changeOpts := &files_service.ChangeRepoFilesOptions{
		OldBranch: defaultBranch,
		NewBranch: defaultBranch,
		Message:   commitMsg,
		Files:     operations,
		Author: &files_service.IdentityOptions{
			GitUserName:  doer.GitName(),
			GitUserEmail: doer.GetEmail(),
		},
		Committer: &files_service.IdentityOptions{
			GitUserName:  doer.GitName(),
			GitUserEmail: doer.GetEmail(),
		},
	}

	_, err = files_service.ChangeRepoFiles(ctx, repo, doer, changeOpts)
	return err
}

func extractZipSafe(zr *zip.Reader, dest string) error {
	for _, file := range zr.File {
		cleanName := filepath.Clean(file.Name)
		if cleanName == "." || cleanName == "" {
			continue
		}
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") || filepath.VolumeName(cleanName) != "" {
			return fmt.Errorf("invalid entry path in archive: %s", file.Name)
		}

		target := filepath.Join(dest, cleanName)
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("archive entry escapes destination: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", cleanName, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", cleanName, err)
		}

		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", cleanName, err)
		}

		if err := writeFile(target, rc, file.FileInfo().Mode()); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
	}
	return nil
}

func writeFile(dst string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create file %s: %w", dst, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file %s: %w", dst, err)
	}
	return nil
}

func determinePackageRoot(tempDir string) (string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return "", fmt.Errorf("read archive contents: %w", err)
	}

	if len(entries) == 1 && entries[0].IsDir() {
		single := filepath.Join(tempDir, entries[0].Name())
		if _, err := os.Stat(filepath.Join(single, "manifest.json")); err == nil {
			return single, nil
		}
	}

	if _, err := os.Stat(filepath.Join(tempDir, "manifest.json")); err == nil {
		return tempDir, nil
	}

	return "", fmt.Errorf("manifest.json is required in the UAPF package")
}

func normalizeTargetPath(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", nil
	}
	clean := path.Clean("/" + target)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." {
		return "", nil
	}
	if strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid target path")
	}
	return clean, nil
}

func buildFileOperations(ctx context.Context, repo *repo_model.Repository, packageRoot, targetPath string) ([]*files_service.ChangeRepoFile, error) {
	ops := make([]*files_service.ChangeRepoFile, 0)
	root := packageRoot

	conflicts := []string{}

	var currentCommit *git.Commit
	if !repo.IsEmpty {
		gr, closer, err := gitrepo.RepositoryFromContextOrOpen(ctx, repo)
		if err != nil {
			return nil, err
		}
		defer closer.Close()
		currentCommit, err = gr.GetBranchCommit(repo.DefaultBranch)
		if err != nil {
			return nil, err
		}
	}

	err := filepath.WalkDir(root, func(pathOnDisk string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, pathOnDisk)
		if err != nil {
			return err
		}
		treePath := path.Join(targetPath, filepath.ToSlash(rel))
		treePath = files_service.CleanGitTreePath(treePath)
		if treePath == "" {
			return fmt.Errorf("invalid path in package: %s", rel)
		}

		if currentCommit != nil {
			if _, err := currentCommit.GetTreeEntryByPath(treePath); err == nil {
				conflicts = append(conflicts, treePath)
				return nil
			} else if !git.IsErrNotExist(err) {
				return err
			}
		}

		content, err := os.Open(pathOnDisk)
		if err != nil {
			return err
		}

		ops = append(ops, &files_service.ChangeRepoFile{
			Operation:     "create",
			TreePath:      treePath,
			ContentReader: content,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(conflicts) > 0 {
		return nil, fmt.Errorf("import would overwrite existing files: %s", strings.Join(conflicts, ", "))
	}

	return ops, nil
}
