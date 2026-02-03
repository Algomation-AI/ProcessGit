// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package processgitviewer

import (
	"encoding/json"
	"fmt"
	"path"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

// LoadManifestFromDir loads processgit.viewer.json from a repo directory.
func LoadManifestFromDir(commit *git.Commit, dir string) (*Manifest, *git.TreeEntry, error) {
	manifestPath := path.Join(dir, "processgit.viewer.json")
	entry, err := commit.GetTreeEntryByPath(manifestPath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	if entry.IsDir() {
		return nil, nil, fmt.Errorf("manifest path %s is a directory", manifestPath)
	}

	if entry.Blob().Size() > setting.UI.MaxDisplayFileSize {
		return nil, nil, fmt.Errorf("manifest %s exceeds max size", manifestPath)
	}

	data, err := entry.Blob().GetBlobContent(setting.UI.MaxDisplayFileSize)
	if err != nil {
		return nil, nil, err
	}

	var manifest Manifest
	if err := json.Unmarshal([]byte(data), &manifest); err != nil {
		return nil, nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, nil, err
	}
	return &manifest, entry, nil
}
