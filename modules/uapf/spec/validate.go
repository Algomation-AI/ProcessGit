// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package spec

import (
	"errors"
	"fmt"
)

// FileChecker abstracts package-content lookups so the same structural checks
// run against an extracted directory (import) or a git tree (export).
type FileChecker interface {
	// DirHasFiles reports whether dir exists and contains at least one file
	// whose name ends with suffix (case-insensitive). An empty suffix matches
	// any file.
	DirHasFiles(dir, suffix string) bool
	// FileExists reports whether the given package-relative file exists.
	FileExists(path string) bool
}

// ValidateManifest performs manifest-internal consistency checks expected by
// UAPF v2.2.0, beyond what the embedded JSON schema already enforces.
func ValidateManifest(m *Manifest) error {
	if m == nil {
		return errors.New("manifest is missing")
	}
	if m.Kind != "uapf.package" {
		return fmt.Errorf("manifest.kind must be \"uapf.package\", got %q", m.Kind)
	}
	if m.ID == "" {
		return errors.New("manifest.id is required")
	}
	if m.Name == "" {
		return errors.New("manifest.name is required")
	}
	if m.Version == "" {
		return errors.New("manifest.version is required")
	}
	if m.Level < 0 || m.Level > 4 {
		return fmt.Errorf("manifest.level must be between 0 and 4, got %d", m.Level)
	}
	if m.Cornerstones == nil {
		return errors.New("manifest.cornerstones is required")
	}
	if m.Level >= 4 && !m.Cornerstones.BPMN {
		return errors.New("a Level-4 package must declare the bpmn cornerstone")
	}
	return nil
}

// ValidatePackageStructure verifies that the package contents match what the
// manifest declares, following the UAPF v2.2.0 package-format rules.
func ValidatePackageStructure(m *Manifest, fc FileChecker) error {
	if m == nil || m.Cornerstones == nil {
		return errors.New("manifest is missing required fields")
	}
	if m.Cornerstones.BPMN && !fc.DirHasFiles(m.BPMNDir(), ".bpmn") {
		return fmt.Errorf("bpmn cornerstone declared but no .bpmn file found in %s/", m.BPMNDir())
	}
	if m.Cornerstones.DMN && !fc.DirHasFiles(m.DMNDir(), ".dmn") {
		return fmt.Errorf("dmn cornerstone declared but no .dmn file found in %s/", m.DMNDir())
	}
	if m.Cornerstones.CMMN && !fc.DirHasFiles(m.CMMNDir(), ".cmmn") {
		return fmt.Errorf("cmmn cornerstone declared but no .cmmn file found in %s/", m.CMMNDir())
	}
	if m.Cornerstones.Resources && !fc.DirHasFiles(m.ResourcesDir(), "") {
		return fmt.Errorf("resources cornerstone declared but %s/ is empty", m.ResourcesDir())
	}
	if m.Level >= 4 {
		mappings := m.ResourcesDir() + "/mappings.yaml"
		if !fc.FileExists(mappings) {
			return fmt.Errorf("a Level-4 package requires %s", mappings)
		}
		for _, f := range []string{"ownership.yaml", "lifecycle.yaml"} {
			p := m.MetadataDir() + "/" + f
			if !fc.FileExists(p) {
				return fmt.Errorf("a Level-4 package requires %s", p)
			}
		}
	}
	return nil
}
