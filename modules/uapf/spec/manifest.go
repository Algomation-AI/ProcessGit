// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package spec

// Manifest describes the structure of a UAPF manifest.json file.
// It mirrors the embedded schema and captures the references we need to validate.
type Manifest struct {
	Name      string            `json:"name"`
	Version   string            `json:"version"`
	Package   *Package          `json:"package"`
	Workflows []ReferencedEntry `json:"workflows"`
	Resources []ReferencedEntry `json:"resources"`
	Metadata  map[string]any    `json:"metadata"`
}

// Package contains optional package metadata fields.
type Package struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Summary     string   `json:"summary"`
	Maintainers []string `json:"maintainers"`
}

// ReferencedEntry represents an item that points to a file within the package.
type ReferencedEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}
