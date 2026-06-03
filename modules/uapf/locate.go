// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/json"

	"gopkg.in/yaml.v3"
)

// ManifestNames lists accepted UAPF manifest file names in priority order.
// UAPF v2.2.0 mandates uapf.yaml; the remaining names are accepted for
// compatibility with UAPF-IP packages and legacy archives. Whatever the file
// name, the contents are validated against the UAPF v2.2.0 manifest schema.
var ManifestNames = []string{
	"uapf.yaml",
	"uapf.yml",
	"process.uapf.yaml",
	"process.uapf.yml",
	"manifest.json",
}

// IsManifestName reports whether base (a file name, not a path) is an accepted
// UAPF manifest file name.
func IsManifestName(base string) bool {
	for _, n := range ManifestNames {
		if base == n {
			return true
		}
	}
	return false
}

// manifestPriority returns the priority index of a manifest base name; a lower
// value is preferred. Unknown names sort last.
func manifestPriority(base string) int {
	for i, n := range ManifestNames {
		if base == n {
			return i
		}
	}
	return len(ManifestNames)
}

// isYAMLName reports whether the file name denotes a YAML document.
func isYAMLName(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".yaml") || strings.HasSuffix(l, ".yml")
}

// manifestToJSON normalizes manifest bytes to JSON, decoding YAML when the file
// name denotes a YAML document. JSON manifests are returned unchanged.
func manifestToJSON(name string, data []byte) ([]byte, error) {
	if !isYAMLName(name) {
		return data, nil
	}
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("%s is not valid YAML: %w", filepath.Base(name), err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("normalize %s: %w", filepath.Base(name), err)
	}
	return out, nil
}
