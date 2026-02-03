// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package processgitviewer

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"
)

// ResolveBinding returns the binding that applies to repoTreePath and also validates
// referenced files exist in the same dir.
func ResolveBinding(commit *git.Commit, dir string, repoTreePath string, manifest *Manifest) (*ViewerBinding, error) {
	if manifest == nil {
		return nil, nil
	}
	for i := range manifest.Viewers {
		binding := &manifest.Viewers[i]
		matched, err := MatchBinding(*binding, repoTreePath)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}

		entryPath := joinFromDir(dir, binding.Entry)
		entry, err := commit.GetTreeEntryByPath(entryPath)
		if err != nil {
			return nil, err
		}
		if entry.IsDir() {
			return nil, fmt.Errorf("entry %s is a directory", entryPath)
		}

		for name, targetPath := range binding.Targets {
			fullPath := joinFromDir(dir, targetPath)
			targetEntry, err := commit.GetTreeEntryByPath(fullPath)
			if err != nil {
				return nil, fmt.Errorf("target %s (%s) missing: %w", name, fullPath, err)
			}
			if targetEntry.IsDir() {
				return nil, fmt.Errorf("target %s (%s) is a directory", name, fullPath)
			}
		}

		editAllowed := false
		for _, editPath := range binding.EditAllow {
			fullPath := joinFromDir(dir, editPath)
			editEntry, err := commit.GetTreeEntryByPath(fullPath)
			if err != nil {
				return nil, fmt.Errorf("edit_allow path %s missing: %w", fullPath, err)
			}
			if editEntry.IsDir() {
				return nil, fmt.Errorf("edit_allow path %s is a directory", fullPath)
			}
			if fullPath == repoTreePath {
				editAllowed = true
			}
		}

		if !editAllowed {
			return nil, fmt.Errorf("edit_allow does not include primary file %s", repoTreePath)
		}

		return binding, nil
	}

	return nil, nil
}
