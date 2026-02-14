// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigFileName is the default name for the chat agent config file.
	DefaultConfigFileName = "agent.chat.yaml"

	// ConfigSuffix is the suffix that identifies chat agent config files.
	ConfigSuffix = ".agent.chat.yaml"

	// ProcessGitConfigDir is the config subdirectory.
	ProcessGitConfigDir = ".processgit"

	maxChatConfigSize int64 = 64 * 1024 // 64 KB
)

// LoadChatConfig loads an agent.chat.yaml from the repository at the given commit.
// It searches using the priority order:
//  1. agent.chat.yaml (root directory)
//  2. .processgit/agent.chat.yaml (config directory)
//  3. *.agent.chat.yaml (any named variant in root)
//
// Returns nil, nil if no config file is found.
func LoadChatConfig(commit *git.Commit, filename string) (*ChatConfig, error) {
	if filename != "" {
		return loadConfigFile(commit, filename)
	}

	// Priority 1: agent.chat.yaml in root
	cfg, err := loadConfigFile(commit, DefaultConfigFileName)
	if cfg != nil || err != nil {
		return cfg, err
	}

	// Priority 2: .processgit/agent.chat.yaml
	cfg, err = loadConfigFile(commit, filepath.Join(ProcessGitConfigDir, DefaultConfigFileName))
	if cfg != nil || err != nil {
		return cfg, err
	}

	return nil, nil
}

// ListChatAgents returns all chat agent configurations found in a repository.
func ListChatAgents(commit *git.Commit) ([]ChatAgentInfo, error) {
	var agents []ChatAgentInfo

	tree, err := commit.SubTree("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get root tree: %w", err)
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return nil, fmt.Errorf("failed to list root entries: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && isChatConfigFile(name) {
			cfg, err := loadConfigFile(commit, name)
			if err != nil {
				continue // skip invalid configs
			}
			if cfg != nil {
				agents = append(agents, ChatAgentInfo{
					FilePath: name,
					Config:   cfg,
				})
			}
		}
	}

	// Check .processgit/ directory
	pgTree, err := commit.SubTree(ProcessGitConfigDir)
	if err == nil {
		pgEntries, err := pgTree.ListEntries()
		if err == nil {
			for _, entry := range pgEntries {
				name := entry.Name()
				if !entry.IsDir() && isChatConfigFile(name) {
					fullPath := filepath.Join(ProcessGitConfigDir, name)
					cfg, err := loadConfigFile(commit, fullPath)
					if err != nil {
						continue
					}
					if cfg != nil {
						agents = append(agents, ChatAgentInfo{
							FilePath: fullPath,
							Config:   cfg,
						})
					}
				}
			}
		}
	}

	return agents, nil
}

// ChatAgentInfo pairs a config file path with its parsed configuration.
type ChatAgentInfo struct {
	FilePath string      `json:"file_path"`
	Config   *ChatConfig `json:"config"`
}

// ResolveAPIKey resolves an API key reference to the actual key value.
// It checks environment variables first.
func ResolveAPIKey(ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("api_key_ref is empty")
	}

	// Priority 1: Environment variable
	if val := os.Getenv(ref); val != "" {
		return val, nil
	}

	// Priority 2: app.ini [chat] section
	if setting.CfgProvider != nil {
		chatSec, err := setting.CfgProvider.GetSection("chat")
		if err == nil && chatSec != nil {
			if key := setting.ConfigSectionKey(chatSec, ref); key != nil && key.String() != "" {
				return key.String(), nil
			}
		}
	}

	// Priority 3: Org-prefixed references (future)
	if strings.HasPrefix(ref, "org:") {
		return "", fmt.Errorf("org-level API key resolution not yet implemented for ref %q", ref)
	}

	return "", fmt.Errorf("API key not found for ref %q: set as environment variable or add to [chat] section in app.ini", ref)
}

func loadConfigFile(commit *git.Commit, filePath string) (*ChatConfig, error) {
	entry, err := commit.GetTreeEntryByPath(filePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", filePath, err)
	}

	if entry.IsDir() {
		return nil, nil
	}
	if entry.Blob().Size() > maxChatConfigSize {
		return nil, fmt.Errorf("%s exceeds max size (%d bytes)", filePath, maxChatConfigSize)
	}

	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, fmt.Errorf("error reading %s blob: %w", filePath, err)
	}
	defer reader.Close()

	var cfg ChatConfig
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", filePath, err)
	}

	if err := validateChatConfig(&cfg); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)

	return &cfg, nil
}

func validateChatConfig(cfg *ChatConfig) error {
	if cfg.UI.Name == "" {
		return fmt.Errorf("agent.chat.yaml: ui.name is required")
	}
	if cfg.LLM.Provider == "" {
		return fmt.Errorf("agent.chat.yaml: llm.provider is required")
	}
	if cfg.LLM.Model == "" {
		return fmt.Errorf("agent.chat.yaml: llm.model is required")
	}
	if cfg.LLM.APIKeyRef == "" {
		return fmt.Errorf("agent.chat.yaml: llm.api_key_ref is required")
	}

	// Validate provider
	switch cfg.LLM.Provider {
	case "anthropic", "openai", "ollama":
		// valid
	default:
		return fmt.Errorf("agent.chat.yaml: llm.provider %q is not supported (must be anthropic, openai, or ollama)", cfg.LLM.Provider)
	}

	return nil
}

func applyDefaults(cfg *ChatConfig) {
	if cfg.Version == "" {
		cfg.Version = "1.0"
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 1024
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.3
	}
	if cfg.UI.Language == "" {
		cfg.UI.Language = "en"
	}
	if cfg.UI.Placeholder == "" {
		cfg.UI.Placeholder = "Ask a question..."
	}
	if cfg.UI.Theme.MaxHeight == "" {
		cfg.UI.Theme.MaxHeight = "600px"
	}
	if cfg.UI.Theme.AssistantAvatar == "" {
		cfg.UI.Theme.AssistantAvatar = "\U0001F916" // robot emoji
	}
	if cfg.UI.Theme.UserAvatar == "" {
		cfg.UI.Theme.UserAvatar = "\U0001F464" // bust emoji
	}
	if cfg.History.Branch == "" {
		cfg.History.Branch = "chat-history"
	}
	if cfg.History.RetentionDays == 0 {
		cfg.History.RetentionDays = 90
	}
	if cfg.History.MaxConversationsPerUser == 0 {
		cfg.History.MaxConversationsPerUser = 100
	}
	if cfg.Access.Visibility == "" {
		cfg.Access.Visibility = "authenticated"
	}
	if cfg.Access.RateLimits.RequestsPerMinute == 0 {
		cfg.Access.RateLimits.RequestsPerMinute = 10
	}
	if cfg.Access.RateLimits.RequestsPerDay == 0 {
		cfg.Access.RateLimits.RequestsPerDay = 100
	}
	if cfg.Access.RateLimits.MaxConversationTurns == 0 {
		cfg.Access.RateLimits.MaxConversationTurns = 50
	}
}

func isChatConfigFile(name string) bool {
	return name == DefaultConfigFileName || strings.HasSuffix(name, ConfigSuffix)
}
