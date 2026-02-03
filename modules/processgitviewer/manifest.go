// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package processgitviewer

import (
	"fmt"
	"path"
	"strings"
)

type Manifest struct {
	Version int             `json:"version"`
	Viewers []ViewerBinding `json:"viewers"`
}

type ViewerBinding struct {
	ID string `json:"id"`

	// Pattern used to decide whether THIS binding applies to the currently viewed file.
	// Must be a glob pattern (Go path.Match semantics).
	// Examples:
	//   "vdvc-register.xml"
	//   "*-register.xml"
	//   "registers/*.xml"
	PrimaryPattern string `json:"primary_pattern"`

	// Viewer type - v1 supports only "html"
	Type string `json:"type"`

	// GUI entry file name/path relative to directory of the manifest
	Entry string `json:"entry"`

	// Exactly which repo-relative file(s) are allowed to be edited by this viewer.
	// In your requirement: only the primary file.
	EditAllow []string `json:"edit_allow"`

	// Optional mapping of “related” files the GUI may need (xsd, examples, etc.)
	// Values are paths relative to the manifest directory.
	Targets map[string]string `json:"targets,omitempty"`
}

func (m *Manifest) Validate() error {
	if m.Version < 1 {
		return fmt.Errorf("manifest version must be >= 1")
	}
	if len(m.Viewers) == 0 {
		return fmt.Errorf("manifest must include at least one viewer binding")
	}
	for i, viewer := range m.Viewers {
		if strings.TrimSpace(viewer.ID) == "" {
			return fmt.Errorf("viewer %d: id is required", i)
		}
		if strings.TrimSpace(viewer.PrimaryPattern) == "" {
			return fmt.Errorf("viewer %d: primary_pattern is required", i)
		}
		if viewer.Type != "html" {
			return fmt.Errorf("viewer %d: type must be html", i)
		}
		if strings.TrimSpace(viewer.Entry) == "" {
			return fmt.Errorf("viewer %d: entry is required", i)
		}
		if len(viewer.EditAllow) == 0 {
			return fmt.Errorf("viewer %d: edit_allow must not be empty", i)
		}
	}
	return nil
}

func MatchBinding(binding ViewerBinding, repoTreePath string) (bool, error) {
	pattern := binding.PrimaryPattern
	if strings.Contains(pattern, "/") {
		return path.Match(pattern, repoTreePath)
	}
	return path.Match(pattern, path.Base(repoTreePath))
}
