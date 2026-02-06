// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_Valid(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server: MCPServerConfig{
			Name:        "Test Server",
			Description: "A test MCP server",
		},
		Sources: []MCPSource{
			{
				Path: "data.xml",
				Type: "xml",
			},
		},
	}
	err := validateConfig(cfg)
	require.NoError(t, err)
}

func TestValidateConfig_InvalidVersion(t *testing.T) {
	cfg := &MCPConfig{
		Version: 2,
		Server:  MCPServerConfig{Name: "Test"},
		Sources: []MCPSource{{Path: "data.xml", Type: "xml"}},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "unsupported version 2")
}

func TestValidateConfig_MissingName(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server:  MCPServerConfig{},
		Sources: []MCPSource{{Path: "data.xml", Type: "xml"}},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "server.name is required")
}

func TestValidateConfig_NoSources(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server:  MCPServerConfig{Name: "Test"},
		Sources: []MCPSource{},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "at least one source is required")
}

func TestValidateConfig_MissingSourcePath(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server:  MCPServerConfig{Name: "Test"},
		Sources: []MCPSource{{Type: "xml"}},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "sources[0].path is required")
}

func TestValidateConfig_MissingSourceType(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server:  MCPServerConfig{Name: "Test"},
		Sources: []MCPSource{{Path: "data.xml"}},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "sources[0].type is required")
}

func TestValidateConfig_UnsupportedSourceType(t *testing.T) {
	cfg := &MCPConfig{
		Version: 1,
		Server:  MCPServerConfig{Name: "Test"},
		Sources: []MCPSource{{Path: "data.json", Type: "json"}},
	}
	err := validateConfig(cfg)
	assert.ErrorContains(t, err, "not supported")
}
