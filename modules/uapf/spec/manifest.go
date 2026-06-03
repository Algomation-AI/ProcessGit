// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package spec

// Manifest mirrors the UAPF v2.2.0 package manifest (uapf.yaml). Only the
// fields ProcessGit consumes are modelled here; full structural validation of
// the manifest is performed against the embedded JSON schema.
type Manifest struct {
	Kind         string        `json:"kind"`
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Level        int           `json:"level"`
	Version      string        `json:"version"`
	Lifecycle    string        `json:"lifecycle"`
	Cornerstones *Cornerstones `json:"cornerstones"`
	Paths        *Paths        `json:"paths"`
}

// Cornerstones records which of the four UAPF cornerstones the package declares.
type Cornerstones struct {
	BPMN      bool `json:"bpmn"`
	DMN       bool `json:"dmn"`
	CMMN      bool `json:"cmmn"`
	Resources bool `json:"resources"`
}

// Paths records optional per-cornerstone directory overrides.
type Paths struct {
	BPMN      string `json:"bpmn"`
	DMN       string `json:"dmn"`
	CMMN      string `json:"cmmn"`
	Resources string `json:"resources"`
	Metadata  string `json:"metadata"`
}

func dirOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// BPMNDir returns the effective bpmn cornerstone directory.
func (m *Manifest) BPMNDir() string {
	if m.Paths != nil {
		return dirOr(m.Paths.BPMN, "bpmn")
	}
	return "bpmn"
}

// DMNDir returns the effective dmn cornerstone directory.
func (m *Manifest) DMNDir() string {
	if m.Paths != nil {
		return dirOr(m.Paths.DMN, "dmn")
	}
	return "dmn"
}

// CMMNDir returns the effective cmmn cornerstone directory.
func (m *Manifest) CMMNDir() string {
	if m.Paths != nil {
		return dirOr(m.Paths.CMMN, "cmmn")
	}
	return "cmmn"
}

// ResourcesDir returns the effective resources cornerstone directory.
func (m *Manifest) ResourcesDir() string {
	if m.Paths != nil {
		return dirOr(m.Paths.Resources, "resources")
	}
	return "resources"
}

// MetadataDir returns the effective metadata directory.
func (m *Manifest) MetadataDir() string {
	if m.Paths != nil {
		return dirOr(m.Paths.Metadata, "metadata")
	}
	return "metadata"
}
