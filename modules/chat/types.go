// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import "time"

// ChatConfig represents the parsed agent.chat.yaml file.
type ChatConfig struct {
	Version string       `yaml:"version"`
	UI      UIConfig     `yaml:"ui"`
	LLM     LLMConfig    `yaml:"llm"`
	MCP     MCPChatConfig `yaml:"mcp"`
	History HistoryConfig `yaml:"history"`
	Access  AccessConfig  `yaml:"access"`
}

// UIConfig holds user interface settings for the chat panel.
type UIConfig struct {
	Name           string      `yaml:"name"`
	Subtitle       string      `yaml:"subtitle"`
	Icon           string      `yaml:"icon"`
	Language       string      `yaml:"language"`
	Placeholder    string      `yaml:"placeholder"`
	WelcomeMessage string      `yaml:"welcome_message"`
	QuickQuestions []string    `yaml:"quick_questions"`
	Theme          ThemeConfig `yaml:"theme"`
}

// ThemeConfig holds visual theme customization.
type ThemeConfig struct {
	PrimaryColor   string `yaml:"primary_color"`
	AssistantAvatar string `yaml:"assistant_avatar"`
	UserAvatar     string `yaml:"user_avatar"`
	MaxHeight      string `yaml:"max_height"`
}

// LLMConfig holds language model backend configuration.
type LLMConfig struct {
	Provider    string  `yaml:"provider"`
	Model       string  `yaml:"model"`
	APIKeyRef   string  `yaml:"api_key_ref"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	TopP        float64 `yaml:"top_p"`
	SystemPrompt string `yaml:"system_prompt"`
}

// MCPChatConfig holds MCP tool configuration for the chat agent.
type MCPChatConfig struct {
	UseRepoMCP        bool              `yaml:"use_repo_mcp"`
	AdditionalServers []MCPServerEntry  `yaml:"additional_servers"`
	AllowedTools      []string          `yaml:"allowed_tools"`
	DeniedTools       []string          `yaml:"denied_tools"`
}

// MCPServerEntry represents an additional MCP server.
type MCPServerEntry struct {
	Name        string `yaml:"name"`
	URL         string `yaml:"url"`
	Description string `yaml:"description"`
}

// HistoryConfig controls conversation persistence.
type HistoryConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	Storage                 string `yaml:"storage"`
	Branch                  string `yaml:"branch"`
	RetentionDays           int    `yaml:"retention_days"`
	MaxConversationsPerUser int    `yaml:"max_conversations_per_user"`
	Anonymize               bool   `yaml:"anonymize"`
}

// AccessConfig controls who can use the chatbot.
type AccessConfig struct {
	Visibility string          `yaml:"visibility"`
	RateLimits RateLimitConfig `yaml:"rate_limits"`
	Budget     BudgetConfig    `yaml:"budget"`
}

// RateLimitConfig defines per-user rate limits.
type RateLimitConfig struct {
	RequestsPerMinute    int `yaml:"requests_per_minute"`
	RequestsPerDay       int `yaml:"requests_per_day"`
	MaxConversationTurns int `yaml:"max_conversation_turns"`
}

// BudgetConfig controls cost limits.
type BudgetConfig struct {
	MaxMonthlyUSD     float64 `yaml:"max_monthly_usd"`
	AlertThresholdPct int     `yaml:"alert_threshold_pct"`
}

// --- Conversation types ---

// Conversation represents a stored chat conversation.
type Conversation struct {
	ID          string           `json:"id"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	User        ConversationUser `json:"user"`
	AgentConfig string           `json:"agent_config"`
	Model       string           `json:"model"`
	Stats       ConversationStats `json:"stats"`
	Messages    []Message        `json:"messages"`
}

// ConversationUser identifies the chat user.
type ConversationUser struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// ConversationStats holds usage statistics for a conversation.
type ConversationStats struct {
	Turns            int      `json:"turns"`
	TotalInputTokens int     `json:"total_input_tokens"`
	TotalOutputTokens int    `json:"total_output_tokens"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	ToolsCalled      []string `json:"tools_called"`
	DurationSeconds  int     `json:"duration_seconds"`
}

// Message represents a single message in a conversation.
type Message struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	Timestamp time.Time  `json:"timestamp"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"`
}

// ToolCall represents an MCP tool invocation within a message.
type ToolCall struct {
	Tool         string `json:"tool"`
	Server       string `json:"server"`
	Query        string `json:"query,omitempty"`
	EntityID     string `json:"entity_id,omitempty"`
	ResultsCount int    `json:"results_count,omitempty"`
}

// Usage tracks token usage for a single response.
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// ConversationSummary is a lightweight representation for listing conversations.
type ConversationSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UserHash  string    `json:"user_hash"`
	CreatedAt time.Time `json:"created_at"`
	Turns     int       `json:"turns"`
	CostUSD   float64   `json:"cost_usd"`
}

// ConversationIndex stores the index of all conversations on the chat-history branch.
type ConversationIndex struct {
	Version            string                `json:"version"`
	TotalConversations int                   `json:"total_conversations"`
	TotalMessages      int                   `json:"total_messages"`
	TotalCostUSD       float64               `json:"total_cost_usd"`
	Conversations      []ConversationSummary `json:"conversations"`
}

// --- Claude API request types ---

// ClaudeRequest represents a request to the Claude Messages API.
type ClaudeRequest struct {
	Model       string            `json:"model"`
	MaxTokens   int               `json:"max_tokens"`
	System      string            `json:"system,omitempty"`
	Messages    []ClaudeMessage   `json:"messages"`
	MCPServers  []ClaudeMCPServer `json:"mcp_servers,omitempty"`
	Tools       []ClaudeTool      `json:"tools,omitempty"`
	Stream      bool              `json:"stream"`
	Temperature float64           `json:"temperature,omitempty"`
}

// ClaudeMessage represents a message in the Claude API format.
type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ClaudeMCPServer represents an MCP server configuration for the Claude API.
type ClaudeMCPServer struct {
	Type               string `json:"type"`
	URL                string `json:"url"`
	Name               string `json:"name"`
	AuthorizationToken string `json:"authorization_token,omitempty"`
}

// ClaudeTool represents a tool configuration for the Claude API.
type ClaudeTool struct {
	Type          string                    `json:"type"`
	MCPServerName string                   `json:"mcp_server_name"`
	DefaultConfig *ClaudeToolDefaultConfig  `json:"default_config,omitempty"`
	Configs       map[string]ClaudeToolOverride `json:"configs,omitempty"`
}

// ClaudeToolDefaultConfig sets default behavior for all tools from an MCP server.
type ClaudeToolDefaultConfig struct {
	Enabled bool `json:"enabled"`
}

// ClaudeToolOverride allows per-tool configuration overrides.
type ClaudeToolOverride struct {
	Enabled bool `json:"enabled"`
}

// --- SSE event types ---

// SSEEvent represents a server-sent event for streaming responses.
type SSEEvent struct {
	Type           string  `json:"type"`
	Text           string  `json:"text,omitempty"`
	Tool           string  `json:"tool,omitempty"`
	Server         string  `json:"server,omitempty"`
	ConversationID string  `json:"conversation_id,omitempty"`
	Usage          *Usage  `json:"usage,omitempty"`
}

// ChatRequest represents the incoming request body for the chat endpoint.
type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
	AgentFile      string `json:"agent_file"`
}
