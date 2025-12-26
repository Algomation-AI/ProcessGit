// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"embed"
	"fmt"
)

//go:embed schemas/uapf-manifest.schema.json
var manifestFiles embed.FS

var manifestSchemaJSON []byte

func init() {
	var err error
	manifestSchemaJSON, err = manifestFiles.ReadFile("schemas/uapf-manifest.schema.json")
	if err != nil {
		panic(fmt.Sprintf("uapf manifest schema missing: %v", err))
	}
}

// ManifestSchema returns the embedded manifest schema content.
func ManifestSchema() []byte {
	return manifestSchemaJSON
}
