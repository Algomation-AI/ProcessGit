// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uapf

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"code.gitea.io/gitea/modules/json"
	uapfresources "code.gitea.io/gitea/resources/uapf"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

var (
	manifestSchema     *jsonschema.Schema
	manifestSchemaOnce sync.Once
	manifestSchemaErr  error
)

func loadManifestSchema() (*jsonschema.Schema, error) {
	manifestSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft2020
		compiler.AddResource("uapf-manifest.schema.json", bytes.NewReader(uapfresources.ManifestSchema()))

		manifestSchema, manifestSchemaErr = compiler.Compile("uapf-manifest.schema.json")
	})

	return manifestSchema, manifestSchemaErr
}

// validateAgainstSchema validates normalized JSON manifest bytes against the
// embedded UAPF v2.2.0 manifest schema.
func validateAgainstSchema(jsonData []byte) error {
	var manifest any
	if err := json.Unmarshal(jsonData, &manifest); err != nil {
		return fmt.Errorf("manifest is not valid JSON: %w", err)
	}

	schema, err := loadManifestSchema()
	if err != nil {
		return fmt.Errorf("load manifest schema: %w", err)
	}

	if err := schema.Validate(manifest); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("manifest validation failed: %s", ve)
		}
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	return nil
}

// ValidatePackage ensures a .uapf archive contains a UAPF manifest (uapf.yaml)
// that conforms to the embedded UAPF v2.2.0 schema.
func ValidatePackage(data []byte) error {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid .uapf archive: %w", err)
	}

	name, raw, err := extractManifest(zipReader)
	if err != nil {
		return err
	}

	jsonData, err := manifestToJSON(name, raw)
	if err != nil {
		return err
	}

	return validateAgainstSchema(jsonData)
}

// extractManifest returns the highest-priority manifest file found anywhere in
// the archive: its base name and raw bytes.
func extractManifest(zipReader *zip.Reader) (string, []byte, error) {
	bestName := ""
	bestPriority := len(ManifestNames)
	var bestFile *zip.File

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(filepath.Clean(file.Name))
		if !IsManifestName(base) {
			continue
		}
		if p := manifestPriority(base); p < bestPriority {
			bestPriority = p
			bestName = base
			bestFile = file
		}
	}

	if bestFile == nil {
		return "", nil, errors.New("a UAPF manifest (uapf.yaml) is required in the package")
	}

	rc, err := bestFile.Open()
	if err != nil {
		return "", nil, fmt.Errorf("open %s: %w", bestName, err)
	}
	defer rc.Close()

	contents, err := io.ReadAll(rc)
	if err != nil {
		return "", nil, fmt.Errorf("read %s: %w", bestName, err)
	}
	return bestName, contents, nil
}

// ValidateManifest validates a manifest file's raw bytes (YAML or JSON,
// selected by name) against the embedded UAPF v2.2.0 schema.
func ValidateManifest(name string, data []byte) error {
	jsonData, err := manifestToJSON(name, data)
	if err != nil {
		return err
	}
	return validateAgainstSchema(jsonData)
}
