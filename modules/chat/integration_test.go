// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildClaudeRequestFormat(t *testing.T) {
	cfg := &ChatConfig{
		Version: "1.0",
		UI:      UIConfig{Name: "Test Assistant"},
		LLM: LLMConfig{
			Provider:     "anthropic",
			Model:        "claude-sonnet-4-5",
			APIKeyRef:    "ANTHROPIC_API_KEY",
			MaxTokens:    1500,
			Temperature:  0.3,
			SystemPrompt: "You are a helpful assistant.",
		},
		MCP: MCPChatConfig{
			UseRepoMCP: true,
			AdditionalServers: []MCPServerEntry{
				{Name: "org-register", URL: "https://example.com/mcp", Description: "Org register"},
			},
			AllowedTools: []string{"search", "get_entity"},
		},
	}

	// Simulate building a request
	conv := NewConversation("agent.chat.yaml", cfg.LLM.Model, "user1", "Test User")
	conv.AddMessage(Message{Role: "user", Content: "Hello"})

	req := &ClaudeRequest{
		Model:       cfg.LLM.Model,
		MaxTokens:   cfg.LLM.MaxTokens,
		System:      cfg.LLM.SystemPrompt,
		Stream:      true,
		Temperature: cfg.LLM.Temperature,
	}

	// Build messages
	for _, msg := range conv.Messages {
		req.Messages = append(req.Messages, ClaudeMessage{Role: msg.Role, Content: msg.Content})
	}

	// Build MCP servers
	if cfg.MCP.UseRepoMCP {
		req.MCPServers = append(req.MCPServers, ClaudeMCPServer{
			Type: "url",
			URL:  "https://processgit.org/owner/repo/mcp",
			Name: "repo-mcp",
		})
	}
	for _, server := range cfg.MCP.AdditionalServers {
		req.MCPServers = append(req.MCPServers, ClaudeMCPServer{
			Type: "url",
			URL:  server.URL,
			Name: server.Name,
		})
	}

	// Build tools with allowlist
	for _, mcpServer := range req.MCPServers {
		tool := ClaudeTool{
			Type:          "mcp_toolset",
			MCPServerName: mcpServer.Name,
		}
		if len(cfg.MCP.AllowedTools) > 0 {
			tool.DefaultConfig = &ClaudeToolDefaultConfig{Enabled: false}
			tool.Configs = make(map[string]ClaudeToolOverride)
			for _, toolName := range cfg.MCP.AllowedTools {
				tool.Configs[toolName] = ClaudeToolOverride{Enabled: true}
			}
		}
		req.Tools = append(req.Tools, tool)
	}

	// Verify request structure
	assert.Equal(t, "claude-sonnet-4-5", req.Model)
	assert.Equal(t, 1500, req.MaxTokens)
	assert.Equal(t, 0.3, req.Temperature)
	assert.True(t, req.Stream)
	assert.Equal(t, "You are a helpful assistant.", req.System)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, "user", req.Messages[0].Role)
	assert.Equal(t, "Hello", req.Messages[0].Content)

	// Verify MCP servers
	assert.Len(t, req.MCPServers, 2)
	assert.Equal(t, "url", req.MCPServers[0].Type)
	assert.Equal(t, "repo-mcp", req.MCPServers[0].Name)
	assert.Equal(t, "org-register", req.MCPServers[1].Name)

	// Verify tools with allowlist
	assert.Len(t, req.Tools, 2)
	for _, tool := range req.Tools {
		assert.Equal(t, "mcp_toolset", tool.Type)
		assert.False(t, tool.DefaultConfig.Enabled) // Default disabled
		assert.True(t, tool.Configs["search"].Enabled)
		assert.True(t, tool.Configs["get_entity"].Enabled)
		_, hasListEntities := tool.Configs["list_entities"]
		assert.False(t, hasListEntities) // Not in allowlist
	}

	// Verify JSON serialization
	jsonBytes, err := json.Marshal(req)
	require.NoError(t, err)

	var parsed map[string]interface{}
	require.NoError(t, json.Unmarshal(jsonBytes, &parsed))

	assert.Equal(t, "claude-sonnet-4-5", parsed["model"])
	assert.Equal(t, true, parsed["stream"])

	mcpServers := parsed["mcp_servers"].([]interface{})
	assert.Len(t, mcpServers, 2)
	firstServer := mcpServers[0].(map[string]interface{})
	assert.Equal(t, "url", firstServer["type"])

	tools := parsed["tools"].([]interface{})
	assert.Len(t, tools, 2)
	firstTool := tools[0].(map[string]interface{})
	assert.Equal(t, "mcp_toolset", firstTool["type"])
}

func TestToolDenyList(t *testing.T) {
	cfg := &ChatConfig{
		MCP: MCPChatConfig{
			DeniedTools: []string{"validate", "identify"},
		},
	}

	tool := ClaudeTool{
		Type:          "mcp_toolset",
		MCPServerName: "test-server",
	}

	if len(cfg.MCP.DeniedTools) > 0 {
		tool.DefaultConfig = &ClaudeToolDefaultConfig{Enabled: true}
		tool.Configs = make(map[string]ClaudeToolOverride)
		for _, toolName := range cfg.MCP.DeniedTools {
			tool.Configs[toolName] = ClaudeToolOverride{Enabled: false}
		}
	}

	assert.True(t, tool.DefaultConfig.Enabled) // Default enabled
	assert.False(t, tool.Configs["validate"].Enabled)
	assert.False(t, tool.Configs["identify"].Enabled)
}

func TestMultiTurnConversation(t *testing.T) {
	conv := NewConversation("agent.chat.yaml", "claude-sonnet-4-5", "user1", "User")

	// Turn 1
	conv.AddMessage(Message{Role: "user", Content: "What is P-1-13?"})
	conv.AddMessage(Message{
		Role:    "assistant",
		Content: "P-1-13 is a classification category...",
		ToolCalls: []ToolCall{
			{Tool: "search", Server: "classification", Query: "P-1-13"},
		},
		Usage: &Usage{InputTokens: 100, OutputTokens: 200, CostUSD: 0.01},
	})

	// Turn 2
	conv.AddMessage(Message{Role: "user", Content: "How does it differ from P-1-14?"})
	conv.AddMessage(Message{
		Role:    "assistant",
		Content: "P-1-14 covers a different area...",
		ToolCalls: []ToolCall{
			{Tool: "get_entity", Server: "classification"},
			{Tool: "get_entity", Server: "classification"},
		},
		Usage: &Usage{InputTokens: 300, OutputTokens: 400, CostUSD: 0.02},
	})

	assert.Equal(t, 4, conv.Stats.Turns)
	assert.Equal(t, 400, conv.Stats.TotalInputTokens)
	assert.Equal(t, 600, conv.Stats.TotalOutputTokens)
	assert.InDelta(t, 0.03, conv.Stats.TotalCostUSD, 0.001)
	assert.Equal(t, []string{"search", "get_entity", "get_entity"}, conv.Stats.ToolsCalled)

	// Build Claude messages from conversation
	var claudeMessages []ClaudeMessage
	for _, msg := range conv.Messages {
		claudeMessages = append(claudeMessages, ClaudeMessage{Role: msg.Role, Content: msg.Content})
	}
	assert.Len(t, claudeMessages, 4)
	assert.Equal(t, "user", claudeMessages[0].Role)
	assert.Equal(t, "assistant", claudeMessages[1].Role)
	assert.Equal(t, "user", claudeMessages[2].Role)
	assert.Equal(t, "assistant", claudeMessages[3].Role)
}

func TestSSEEventSerialization(t *testing.T) {
	events := []SSEEvent{
		{Type: "text", Text: "Hello world"},
		{Type: "tool_call", Tool: "search", Server: "classification"},
		{Type: "done", ConversationID: "conv_abc123", Usage: &Usage{InputTokens: 100, OutputTokens: 50, CostUSD: 0.005}},
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		require.NoError(t, err)

		var parsed map[string]interface{}
		require.NoError(t, json.Unmarshal(data, &parsed))

		assert.Equal(t, event.Type, parsed["type"])
	}

	// Verify text event
	data, _ := json.Marshal(events[0])
	var textEvent map[string]interface{}
	json.Unmarshal(data, &textEvent)
	assert.Equal(t, "Hello world", textEvent["text"])

	// Verify done event has usage
	data, _ = json.Marshal(events[2])
	var doneEvent map[string]interface{}
	json.Unmarshal(data, &doneEvent)
	assert.Equal(t, "conv_abc123", doneEvent["conversation_id"])
	usage := doneEvent["usage"].(map[string]interface{})
	assert.Equal(t, float64(100), usage["input_tokens"])
}

func TestConversationJSONRoundTrip(t *testing.T) {
	conv := NewConversation("agent.chat.yaml", "claude-sonnet-4-5", "user123", "Test User")
	conv.AddMessage(Message{Role: "user", Content: "Hello"})
	conv.AddMessage(Message{
		Role:      "assistant",
		Content:   "Hi there!",
		ToolCalls: []ToolCall{{Tool: "search", Server: "test", ResultsCount: 3}},
		Usage:     &Usage{InputTokens: 50, OutputTokens: 30, CostUSD: 0.002},
	})

	// Serialize
	data, err := json.Marshal(conv)
	require.NoError(t, err)

	// Deserialize
	var restored Conversation
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, conv.ID, restored.ID)
	assert.Equal(t, conv.Model, restored.Model)
	assert.Equal(t, conv.User.ID, restored.User.ID)
	assert.Len(t, restored.Messages, 2)
	assert.Equal(t, "user", restored.Messages[0].Role)
	assert.Equal(t, "Hello", restored.Messages[0].Content)
	assert.Equal(t, "assistant", restored.Messages[1].Role)
	assert.Len(t, restored.Messages[1].ToolCalls, 1)
	assert.Equal(t, "search", restored.Messages[1].ToolCalls[0].Tool)
	assert.Equal(t, 3, restored.Messages[1].ToolCalls[0].ResultsCount)
	assert.InDelta(t, 0.002, restored.Messages[1].Usage.CostUSD, 0.0001)
}

func TestEstimateCost(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int
		outputTokens int
		model        string
		minCost      float64
		maxCost      float64
	}{
		{"Sonnet small", 100, 50, "claude-sonnet-4-5", 0.0001, 0.002},
		{"Opus small", 100, 50, "claude-opus-4-6", 0.0001, 0.005},
		{"Haiku small", 100, 50, "claude-haiku-4-5", 0.00001, 0.001},
		{"Unknown model", 100, 50, "unknown-model", 0.0001, 0.002},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := EstimateCost(tt.inputTokens, tt.outputTokens, tt.model)
			assert.Greater(t, cost, tt.minCost)
			assert.Less(t, cost, tt.maxCost)
		})
	}
}

// EstimateCost is exported for testing; mirrors the handler's estimateCost function.
func EstimateCost(inputTokens, outputTokens int, model string) float64 {
	var inputRate, outputRate float64
	switch {
	case contains(model, "opus"):
		inputRate = 5.0
		outputRate = 25.0
	case contains(model, "sonnet"):
		inputRate = 3.0
		outputRate = 15.0
	case contains(model, "haiku"):
		inputRate = 0.25
		outputRate = 1.25
	default:
		inputRate = 3.0
		outputRate = 15.0
	}
	return (float64(inputTokens)*inputRate + float64(outputTokens)*outputRate) / 1_000_000
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
