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

// ValidatePackage ensures a .uapf archive contains a manifest.json that conforms to the embedded schema.
func ValidatePackage(data []byte) error {
	readerAt := bytes.NewReader(data)

	zipReader, err := zip.NewReader(readerAt, int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid .uapf archive: %w", err)
	}

	manifestJSON, err := extractManifest(zipReader)
	if err != nil {
		return err
	}

	var manifest any
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return fmt.Errorf("manifest.json is not valid JSON: %w", err)
	}

	schema, err := loadManifestSchema()
	if err != nil {
		return fmt.Errorf("load manifest schema: %w", err)
	}

	if err := schema.Validate(manifest); err != nil {
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			return fmt.Errorf("manifest validation failed: %s", validationErr)
		}
		return fmt.Errorf("manifest validation failed: %w", err)
	}

	return nil
}

func extractManifest(zipReader *zip.Reader) ([]byte, error) {
	for _, file := range zipReader.File {
		name := filepath.Clean(file.Name)
		if filepath.Base(name) != "manifest.json" {
			continue
		}

		manifestReader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open manifest.json: %w", err)
		}
		defer manifestReader.Close()

		contents, err := io.ReadAll(manifestReader)
		if err != nil {
			return nil, fmt.Errorf("read manifest.json: %w", err)
		}
		return contents, nil
	}

	return nil, errors.New("manifest.json is required in the UAPF package")
}

// ValidateManifest validates manifest.json contents against the embedded schema.
func ValidateManifest(data []byte) error {
	var manifest any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("manifest.json is not valid JSON: %w", err)
	}

	schema, err := loadManifestSchema()
	if err != nil {
		return fmt.Errorf("load manifest schema: %w", err)
	}

	if err := schema.Validate(manifest); err != nil {
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			return fmt.Errorf("manifest validation failed: %s", validationErr)
		}
		return fmt.Errorf("manifest validation failed: %w", err)
	}
	return nil
}
