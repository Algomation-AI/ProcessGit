// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"

	"code.gitea.io/gitea/modules/git"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the expected name for the MCP config file in the repo root.
const ConfigFileName = "processgit.mcp.yaml"

const maxConfigSize int64 = 64 * 1024 // 64 KB

// LoadConfig loads processgit.mcp.yaml from the repo root at the given commit.
// Returns nil, nil if the file doesn't exist (MCP not enabled for this repo).
func LoadConfig(commit *git.Commit) (*MCPConfig, error) {
	entry, err := commit.GetTreeEntryByPath(ConfigFileName)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", ConfigFileName, err)
	}

	if entry.IsDir() {
		return nil, fmt.Errorf("%s is a directory", ConfigFileName)
	}
	if entry.Blob().Size() > maxConfigSize {
		return nil, fmt.Errorf("%s exceeds max size (%d bytes)", ConfigFileName, maxConfigSize)
	}

	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, fmt.Errorf("error reading %s blob: %w", ConfigFileName, err)
	}
	defer reader.Close()

	var cfg MCPConfig
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", ConfigFileName, err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateConfig(cfg *MCPConfig) error {
	if cfg.Version != 1 {
		return fmt.Errorf("%s: unsupported version %d (expected 1)", ConfigFileName, cfg.Version)
	}
	if cfg.Server.Name == "" {
		return fmt.Errorf("%s: server.name is required", ConfigFileName)
	}
	if len(cfg.Sources) == 0 {
		return fmt.Errorf("%s: at least one source is required", ConfigFileName)
	}

	for i, src := range cfg.Sources {
		if src.Path == "" {
			return fmt.Errorf("%s: sources[%d].path is required", ConfigFileName, i)
		}
		if src.Type == "" {
			return fmt.Errorf("%s: sources[%d].type is required", ConfigFileName, i)
		}
		// For MVP, only "xml" type is supported
		if src.Type != "xml" {
			return fmt.Errorf("%s: sources[%d].type %q is not supported (must be \"xml\")", ConfigFileName, i, src.Type)
		}
	}

	return nil
}
