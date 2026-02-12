// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/chat"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

const (
	anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion  = "2023-06-01"
	anthropicMCPBeta     = "mcp-client-2025-11-20"
)

// rateLimitEntry tracks per-user rate limit state.
type rateLimitEntry struct {
	mu          sync.Mutex
	minuteCount int
	dayCount    int
	minuteReset time.Time
	dayReset    time.Time
}

var (
	rateLimits   sync.Map // key: "repoID:userID" -> *rateLimitEntry
	monthlyCost  sync.Map // key: repoID -> *monthlyCostTracker
)

type monthlyCostTracker struct {
	mu       sync.Mutex
	month    time.Month
	year     int
	totalUSD float64
}

// ChatEndpoint handles chat requests for a repository's agent.chat.yaml.
func ChatEndpoint(ctx *context.Context) {
	if !setting.Chat.Enabled {
		ctx.JSON(http.StatusNotFound, map[string]string{"error": "Chat agents are disabled on this instance"})
		return
	}

	// Parse request body
	var req chat.ChatRequest
	if err := json.NewDecoder(ctx.Req.Body).Decode(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body: " + err.Error()})
		return
	}

	if strings.TrimSpace(req.Message) == "" {
		ctx.JSON(http.StatusBadRequest, map[string]string{"error": "message is required"})
		return
	}

	// Get default branch commit
	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusNotFound, map[string]string{"error": "repository is empty"})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	// Load chat config
	agentFile := req.AgentFile
	if agentFile == "" {
		agentFile = chat.DefaultConfigFileName
	}
	cfg, err := chat.LoadChatConfig(commit, agentFile)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to load chat config: " + err.Error(),
		})
		return
	}
	if cfg == nil {
		ctx.JSON(http.StatusNotFound, map[string]string{
			"error": "no chat agent found (no agent.chat.yaml)",
		})
		return
	}

	// Resolve API key
	apiKey, err := chat.ResolveAPIKey(cfg.LLM.APIKeyRef)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to resolve API key: " + err.Error(),
		})
		return
	}

	// Check rate limits
	userID := "anonymous"
	userName := "Anonymous"
	if ctx.Doer != nil {
		userID = fmt.Sprintf("%d", ctx.Doer.ID)
		userName = ctx.Doer.Name
	}

	if !checkRateLimit(ctx.Repo.Repository.ID, userID, cfg.Access.RateLimits) {
		ctx.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "rate limit exceeded",
		})
		return
	}

	// Check budget
	if cfg.Access.Budget.MaxMonthlyUSD > 0 {
		if !checkBudget(ctx.Repo.Repository.ID, cfg.Access.Budget.MaxMonthlyUSD) {
			ctx.JSON(http.StatusPaymentRequired, map[string]string{
				"error": "monthly budget exceeded",
			})
			return
		}
	}

	// Load or create conversation
	var conv *chat.Conversation
	if req.ConversationID != "" {
		historyBranch := cfg.History.Branch
		if historyBranch == "" {
			historyBranch = "chat-history"
		}
		historyCommit, err := ctx.Repo.GitRepo.GetBranchCommit(historyBranch)
		if err == nil {
			conv, _ = chat.LoadConversation(historyCommit, req.ConversationID)
		}
	}
	if conv == nil {
		conv = chat.NewConversation(agentFile, cfg.LLM.Model, userID, userName)
	}

	// Add user message
	conv.AddMessage(chat.Message{
		Role:      "user",
		Content:   req.Message,
		Timestamp: time.Now().UTC(),
	})

	// Build Claude API request
	claudeReq := buildClaudeRequest(cfg, conv, ctx.Repo.Repository.OwnerName, ctx.Repo.Repository.Name)

	// Stream response via SSE
	ctx.Resp.Header().Set("Content-Type", "text/event-stream")
	ctx.Resp.Header().Set("Cache-Control", "no-cache")
	ctx.Resp.Header().Set("Connection", "keep-alive")
	ctx.Resp.Header().Set("X-Accel-Buffering", "no")

	assistantContent, toolCalls, usage, err := streamClaudeResponse(ctx.Resp, apiKey, claudeReq)
	if err != nil {
		log.Error("Chat streaming error: %v", err)
		writeSSEEvent(ctx.Resp, "error", chat.SSEEvent{Type: "error", Text: err.Error()})
		return
	}

	// Add assistant response to conversation
	assistantMsg := chat.Message{
		Role:      "assistant",
		Content:   assistantContent,
		Timestamp: time.Now().UTC(),
		ToolCalls: toolCalls,
		Usage:     usage,
	}
	conv.AddMessage(assistantMsg)

	// Send completion event
	writeSSEEvent(ctx.Resp, "message_complete", chat.SSEEvent{
		Type:           "done",
		ConversationID: conv.ID,
		Usage:          usage,
	})

	// Track cost
	if usage != nil {
		trackCost(ctx.Repo.Repository.ID, usage.CostUSD)
	}

	// Buffer conversation for async persistence
	if cfg.History.Enabled {
		buf := chat.GetBuffer(ctx.Repo.Repository.ID)
		buf.BufferConversation(conv)
	}
}

// ChatAgents returns a list of chat agents found in the repository.
func ChatAgents(ctx *context.Context) {
	if !setting.Chat.Enabled {
		ctx.JSON(http.StatusNotFound, map[string]string{"error": "Chat agents are disabled"})
		return
	}

	commit, err := ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusOK, []chat.ChatAgentInfo{})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	agents, err := chat.ListChatAgents(commit)
	if err != nil {
		ctx.ServerError("ListChatAgents", err)
		return
	}

	ctx.JSON(http.StatusOK, agents)
}

// ChatHistory returns conversation list for the current user.
func ChatHistory(ctx *context.Context) {
	if !setting.Chat.Enabled {
		ctx.JSON(http.StatusNotFound, map[string]string{"error": "Chat agents are disabled"})
		return
	}

	branch := ctx.FormString("branch")
	if branch == "" {
		branch = "chat-history"
	}

	historyCommit, err := ctx.Repo.GitRepo.GetBranchCommit(branch)
	if err != nil {
		if git.IsErrNotExist(err) {
			ctx.JSON(http.StatusOK, []chat.ConversationSummary{})
		} else {
			ctx.ServerError("GetBranchCommit", err)
		}
		return
	}

	userID := ""
	if ctx.Doer != nil {
		userID = fmt.Sprintf("%d", ctx.Doer.ID)
	}

	limit := ctx.FormInt("limit")
	if limit <= 0 {
		limit = 20
	}
	offset := ctx.FormInt("offset")

	conversations, err := chat.ListConversations(historyCommit, userID, limit, offset)
	if err != nil {
		ctx.ServerError("ListConversations", err)
		return
	}

	ctx.JSON(http.StatusOK, conversations)
}

func buildClaudeRequest(cfg *chat.ChatConfig, conv *chat.Conversation, owner, repoName string) *chat.ClaudeRequest {
	// Build messages from conversation history
	messages := make([]chat.ClaudeMessage, 0, len(conv.Messages))
	for _, msg := range conv.Messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			messages = append(messages, chat.ClaudeMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	req := &chat.ClaudeRequest{
		Model:       cfg.LLM.Model,
		MaxTokens:   cfg.LLM.MaxTokens,
		System:      cfg.LLM.SystemPrompt,
		Messages:    messages,
		Stream:      true,
		Temperature: cfg.LLM.Temperature,
	}

	// Build MCP server configurations
	if cfg.MCP.UseRepoMCP {
		mcpURL := fmt.Sprintf("%s%s/%s/mcp", setting.AppURL, owner, repoName)
		req.MCPServers = append(req.MCPServers, chat.ClaudeMCPServer{
			Type: "url",
			URL:  mcpURL,
			Name: repoName + "-mcp",
		})
	}

	for _, server := range cfg.MCP.AdditionalServers {
		req.MCPServers = append(req.MCPServers, chat.ClaudeMCPServer{
			Type: "url",
			URL:  server.URL,
			Name: server.Name,
		})
	}

	// Build tool configurations
	for _, mcpServer := range req.MCPServers {
		tool := chat.ClaudeTool{
			Type:          "mcp_toolset",
			MCPServerName: mcpServer.Name,
		}

		// Apply tool allow/deny lists
		if len(cfg.MCP.AllowedTools) > 0 {
			// Default all tools to disabled, enable only allowed ones
			tool.DefaultConfig = &chat.ClaudeToolDefaultConfig{Enabled: false}
			tool.Configs = make(map[string]chat.ClaudeToolOverride)
			for _, toolName := range cfg.MCP.AllowedTools {
				tool.Configs[toolName] = chat.ClaudeToolOverride{Enabled: true}
			}
		} else if len(cfg.MCP.DeniedTools) > 0 {
			// Default all tools to enabled, disable denied ones
			tool.DefaultConfig = &chat.ClaudeToolDefaultConfig{Enabled: true}
			tool.Configs = make(map[string]chat.ClaudeToolOverride)
			for _, toolName := range cfg.MCP.DeniedTools {
				tool.Configs[toolName] = chat.ClaudeToolOverride{Enabled: false}
			}
		}

		req.Tools = append(req.Tools, tool)
	}

	return req
}

func streamClaudeResponse(w http.ResponseWriter, apiKey string, req *chat.ClaudeRequest) (string, []chat.ToolCall, *chat.Usage, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", anthropicMessagesURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
	httpReq.Header.Set("anthropic-beta", anthropicMCPBeta)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", nil, nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse SSE stream from Claude
	var fullContent strings.Builder
	var toolCalls []chat.ToolCall
	usage := &chat.Usage{}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)
		switch eventType {
		case "content_block_delta":
			delta, ok := event["delta"].(map[string]interface{})
			if !ok {
				continue
			}
			deltaType, _ := delta["type"].(string)
			if deltaType == "text_delta" {
				text, _ := delta["text"].(string)
				fullContent.WriteString(text)
				writeSSEEvent(w, "message_delta", chat.SSEEvent{Type: "text", Text: text})
			}

		case "content_block_start":
			block, ok := event["content_block"].(map[string]interface{})
			if !ok {
				continue
			}
			blockType, _ := block["type"].(string)
			if blockType == "mcp_tool_use" {
				toolName, _ := block["name"].(string)
				serverName, _ := block["server_name"].(string)
				toolCalls = append(toolCalls, chat.ToolCall{
					Tool:   toolName,
					Server: serverName,
				})
				writeSSEEvent(w, "tool_use", chat.SSEEvent{
					Type:   "tool_call",
					Tool:   toolName,
					Server: serverName,
				})
			}

		case "message_delta":
			if u, ok := event["usage"].(map[string]interface{}); ok {
				if v, ok := u["output_tokens"].(float64); ok {
					usage.OutputTokens = int(v)
				}
			}

		case "message_start":
			if msg, ok := event["message"].(map[string]interface{}); ok {
				if u, ok := msg["usage"].(map[string]interface{}); ok {
					if v, ok := u["input_tokens"].(float64); ok {
						usage.InputTokens = int(v)
					}
				}
			}
		}
	}

	// Calculate approximate cost (Claude Sonnet pricing as default)
	usage.CostUSD = estimateCost(usage.InputTokens, usage.OutputTokens, req.Model)

	return fullContent.String(), toolCalls, usage, nil
}

func writeSSEEvent(w http.ResponseWriter, event string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func estimateCost(inputTokens, outputTokens int, model string) float64 {
	// Approximate pricing per million tokens
	var inputRate, outputRate float64
	switch {
	case strings.Contains(model, "opus"):
		inputRate = 15.0
		outputRate = 75.0
	case strings.Contains(model, "sonnet"):
		inputRate = 3.0
		outputRate = 15.0
	case strings.Contains(model, "haiku"):
		inputRate = 0.25
		outputRate = 1.25
	default:
		inputRate = 3.0
		outputRate = 15.0
	}

	return (float64(inputTokens)*inputRate + float64(outputTokens)*outputRate) / 1_000_000
}

func checkRateLimit(repoID int64, userID string, limits chat.RateLimitConfig) bool {
	key := fmt.Sprintf("%d:%s", repoID, userID)
	val, _ := rateLimits.LoadOrStore(key, &rateLimitEntry{
		minuteReset: time.Now().Add(time.Minute),
		dayReset:    time.Now().Add(24 * time.Hour),
	})
	entry := val.(*rateLimitEntry)

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()

	// Reset counters if window expired
	if now.After(entry.minuteReset) {
		entry.minuteCount = 0
		entry.minuteReset = now.Add(time.Minute)
	}
	if now.After(entry.dayReset) {
		entry.dayCount = 0
		entry.dayReset = now.Add(24 * time.Hour)
	}

	// Check limits
	if limits.RequestsPerMinute > 0 && entry.minuteCount >= limits.RequestsPerMinute {
		return false
	}
	if limits.RequestsPerDay > 0 && entry.dayCount >= limits.RequestsPerDay {
		return false
	}

	entry.minuteCount++
	entry.dayCount++
	return true
}

func checkBudget(repoID int64, maxMonthlyUSD float64) bool {
	val, _ := monthlyCost.LoadOrStore(repoID, &monthlyCostTracker{})
	tracker := val.(*monthlyCostTracker)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	now := time.Now()
	if tracker.month != now.Month() || tracker.year != now.Year() {
		tracker.month = now.Month()
		tracker.year = now.Year()
		tracker.totalUSD = 0
	}

	return tracker.totalUSD < maxMonthlyUSD
}

func trackCost(repoID int64, costUSD float64) {
	val, _ := monthlyCost.LoadOrStore(repoID, &monthlyCostTracker{})
	tracker := val.(*monthlyCostTracker)

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	now := time.Now()
	if tracker.month != now.Month() || tracker.year != now.Year() {
		tracker.month = now.Month()
		tracker.year = now.Year()
		tracker.totalUSD = 0
	}
	tracker.totalUSD += costUSD
}
