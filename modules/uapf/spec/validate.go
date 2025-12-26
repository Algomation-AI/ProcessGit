// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package spec

import (
	"errors"
	"fmt"
	"path"
)

// ValidateManifest performs lightweight structural checks expected by the UAPF schema
// and returns a normalized list of referenced paths to verify on disk.
func ValidateManifest(manifest *Manifest) ([]string, error) {
	if manifest == nil {
		return nil, errors.New("manifest is missing")
	}

	if manifest.Name == "" || manifest.Version == "" {
		if manifest.Package == nil || manifest.Package.Name == "" || manifest.Package.Version == "" {
			return nil, errors.New("manifest must include name and version or package.name and package.version")
		}
	}

	refPaths := make([]string, 0, len(manifest.Workflows)+len(manifest.Resources))
	for _, wf := range manifest.Workflows {
		if wf.Path == "" {
			return nil, errors.New("workflows entry is missing path")
		}
		refPaths = append(refPaths, cleanRelativePath(wf.Path))
	}
	for _, res := range manifest.Resources {
		if res.Path == "" {
			return nil, errors.New("resources entry is missing path")
		}
		refPaths = append(refPaths, cleanRelativePath(res.Path))
	}

	return refPaths, nil
}

func cleanRelativePath(p string) string {
	clean := path.Clean("/" + p)
	return clean[1:]
}
