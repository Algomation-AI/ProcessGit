// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateChatConfig(t *testing.T) {
	t.Run("ValidFullConfig", func(t *testing.T) {
		cfg := &ChatConfig{
			Version: "1.0",
			UI:      UIConfig{Name: "Test Assistant"},
			LLM: LLMConfig{
				Provider:  "anthropic",
				Model:     "claude-sonnet-4-5",
				APIKeyRef: "ANTHROPIC_API_KEY",
			},
		}
		assert.NoError(t, validateChatConfig(cfg))
	})

	t.Run("MissingUIName", func(t *testing.T) {
		cfg := &ChatConfig{
			LLM: LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5", APIKeyRef: "KEY"},
		}
		err := validateChatConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ui.name is required")
	})

	t.Run("MissingProvider", func(t *testing.T) {
		cfg := &ChatConfig{
			UI:  UIConfig{Name: "Test"},
			LLM: LLMConfig{Model: "claude-sonnet-4-5", APIKeyRef: "KEY"},
		}
		err := validateChatConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "llm.provider is required")
	})

	t.Run("MissingModel", func(t *testing.T) {
		cfg := &ChatConfig{
			UI:  UIConfig{Name: "Test"},
			LLM: LLMConfig{Provider: "anthropic", APIKeyRef: "KEY"},
		}
		err := validateChatConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "llm.model is required")
	})

	t.Run("MissingAPIKeyRef", func(t *testing.T) {
		cfg := &ChatConfig{
			UI:  UIConfig{Name: "Test"},
			LLM: LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5"},
		}
		err := validateChatConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "llm.api_key_ref is required")
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		cfg := &ChatConfig{
			UI:  UIConfig{Name: "Test"},
			LLM: LLMConfig{Provider: "invalid", Model: "test", APIKeyRef: "KEY"},
		}
		err := validateChatConfig(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})
}

func TestApplyDefaults(t *testing.T) {
	cfg := &ChatConfig{
		UI:  UIConfig{Name: "Test"},
		LLM: LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5", APIKeyRef: "KEY"},
	}
	applyDefaults(cfg)

	assert.Equal(t, "1.0", cfg.Version)
	assert.Equal(t, 1024, cfg.LLM.MaxTokens)
	assert.Equal(t, 0.3, cfg.LLM.Temperature)
	assert.Equal(t, "en", cfg.UI.Language)
	assert.Equal(t, "Ask a question...", cfg.UI.Placeholder)
	assert.Equal(t, "600px", cfg.UI.Theme.MaxHeight)
	assert.Equal(t, "chat-history", cfg.History.Branch)
	assert.Equal(t, 90, cfg.History.RetentionDays)
	assert.Equal(t, "authenticated", cfg.Access.Visibility)
	assert.Equal(t, 10, cfg.Access.RateLimits.RequestsPerMinute)
	assert.Equal(t, 100, cfg.Access.RateLimits.RequestsPerDay)
}

func TestResolveAPIKey(t *testing.T) {
	t.Run("EmptyRef", func(t *testing.T) {
		_, err := ResolveAPIKey("")
		assert.Error(t, err)
	})

	t.Run("FromEnvVar", func(t *testing.T) {
		t.Setenv("TEST_CHAT_API_KEY", "sk-test-123")
		key, err := ResolveAPIKey("TEST_CHAT_API_KEY")
		assert.NoError(t, err)
		assert.Equal(t, "sk-test-123", key)
	})

	t.Run("MissingEnvVar", func(t *testing.T) {
		_, err := ResolveAPIKey("NONEXISTENT_KEY_12345")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("OrgPrefix", func(t *testing.T) {
		_, err := ResolveAPIKey("org:varam:anthropic_key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not yet implemented")
	})
}

func TestIsChatConfigFile(t *testing.T) {
	assert.True(t, isChatConfigFile("agent.chat.yaml"))
	assert.True(t, isChatConfigFile("classification.agent.chat.yaml"))
	assert.True(t, isChatConfigFile("my-bot.agent.chat.yaml"))
	assert.False(t, isChatConfigFile("agent.yaml"))
	assert.False(t, isChatConfigFile("chat.yaml"))
	assert.False(t, isChatConfigFile("processgit.mcp.yaml"))
}
