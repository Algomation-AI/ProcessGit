// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/git"
)

const (
	indexFileName         = "_index.json"
	defaultHistoryBranch  = "chat-history"
	maxTitleLength        = 60
	batchFlushInterval    = 5 * time.Minute
	batchFlushThreshold   = 10
)

// ConversationBuffer holds conversations pending commit to git.
type ConversationBuffer struct {
	mu            sync.Mutex
	conversations map[string]*Conversation // keyed by conversation ID
	lastFlush     time.Time
	repoID        int64
}

var (
	buffersMu sync.RWMutex
	buffers   = make(map[int64]*ConversationBuffer) // keyed by repo ID
)

// GetBuffer returns the conversation buffer for a repository, creating one if needed.
func GetBuffer(repoID int64) *ConversationBuffer {
	buffersMu.RLock()
	buf, ok := buffers[repoID]
	buffersMu.RUnlock()
	if ok {
		return buf
	}

	buffersMu.Lock()
	defer buffersMu.Unlock()
	// Double-check after acquiring write lock
	if buf, ok := buffers[repoID]; ok {
		return buf
	}
	buf = &ConversationBuffer{
		conversations: make(map[string]*Conversation),
		lastFlush:     time.Now(),
		repoID:        repoID,
	}
	buffers[repoID] = buf
	return buf
}

// BufferConversation adds or updates a conversation in the write buffer.
func (b *ConversationBuffer) BufferConversation(conv *Conversation) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conversations[conv.ID] = conv
}

// ShouldFlush returns true if the buffer should be flushed to git.
func (b *ConversationBuffer) ShouldFlush() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.conversations) == 0 {
		return false
	}
	return len(b.conversations) >= batchFlushThreshold ||
		time.Since(b.lastFlush) >= batchFlushInterval
}

// DrainConversations returns all buffered conversations and clears the buffer.
func (b *ConversationBuffer) DrainConversations() []*Conversation {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.conversations) == 0 {
		return nil
	}
	result := make([]*Conversation, 0, len(b.conversations))
	for _, conv := range b.conversations {
		result = append(result, conv)
	}
	b.conversations = make(map[string]*Conversation)
	b.lastFlush = time.Now()
	return result
}

// GenerateConversationID creates a new unique conversation identifier.
func GenerateConversationID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "conv_" + hex.EncodeToString(b)
}

// ConversationFilePath returns the git path for a conversation file.
func ConversationFilePath(conv *Conversation) string {
	t := conv.CreatedAt
	return fmt.Sprintf("%d/%02d/%02d/%s.json", t.Year(), t.Month(), t.Day(), conv.ID)
}

// GenerateTitle creates a conversation title from the first user message.
func GenerateTitle(conv *Conversation) string {
	for _, msg := range conv.Messages {
		if msg.Role == "user" {
			title := msg.Content
			if len(title) > maxTitleLength {
				title = title[:maxTitleLength] + "..."
			}
			// Remove newlines
			title = strings.ReplaceAll(title, "\n", " ")
			title = strings.TrimSpace(title)
			return title
		}
	}
	return "New conversation"
}

// LoadConversation reads a conversation from the chat-history branch by ID.
// Returns nil, nil if the conversation is not found.
func LoadConversation(commit *git.Commit, convID string) (*Conversation, error) {
	// Load index to find the conversation path
	index, err := LoadIndex(commit)
	if err != nil {
		return nil, err
	}
	if index == nil {
		return nil, nil
	}

	// Find the conversation in the index to get its creation date for the path
	for _, summary := range index.Conversations {
		if summary.ID == convID {
			t := summary.CreatedAt
			path := fmt.Sprintf("%d/%02d/%02d/%s.json", t.Year(), t.Month(), t.Day(), convID)
			return loadConversationByPath(commit, path)
		}
	}

	return nil, nil
}

func loadConversationByPath(commit *git.Commit, filePath string) (*Conversation, error) {
	entry, err := commit.GetTreeEntryByPath(filePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading conversation %s: %w", filePath, err)
	}

	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, fmt.Errorf("error reading conversation blob %s: %w", filePath, err)
	}
	defer reader.Close()

	var conv Conversation
	if err := json.NewDecoder(reader).Decode(&conv); err != nil {
		return nil, fmt.Errorf("invalid conversation JSON %s: %w", filePath, err)
	}

	return &conv, nil
}

// LoadIndex reads the _index.json from the chat-history branch.
func LoadIndex(commit *git.Commit) (*ConversationIndex, error) {
	entry, err := commit.GetTreeEntryByPath(indexFileName)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading %s: %w", indexFileName, err)
	}

	reader, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, fmt.Errorf("error reading %s blob: %w", indexFileName, err)
	}
	defer reader.Close()

	var index ConversationIndex
	if err := json.NewDecoder(reader).Decode(&index); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", indexFileName, err)
	}

	return &index, nil
}

// ListConversations returns conversation summaries with pagination.
func ListConversations(commit *git.Commit, userID string, limit, offset int) ([]ConversationSummary, error) {
	index, err := LoadIndex(commit)
	if err != nil {
		return nil, err
	}
	if index == nil {
		return nil, nil
	}

	var filtered []ConversationSummary
	for _, summary := range index.Conversations {
		if userID == "" || summary.UserHash == userID {
			filtered = append(filtered, summary)
		}
	}

	// Apply pagination
	if offset >= len(filtered) {
		return nil, nil
	}
	filtered = filtered[offset:]
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// BuildUpdatedIndex creates an updated index incorporating new/modified conversations.
func BuildUpdatedIndex(existing *ConversationIndex, conversations []*Conversation) *ConversationIndex {
	if existing == nil {
		existing = &ConversationIndex{
			Version:       "1.0",
			Conversations: make([]ConversationSummary, 0),
		}
	}

	// Build a map of existing conversations for quick lookup
	existingMap := make(map[string]int)
	for i, conv := range existing.Conversations {
		existingMap[conv.ID] = i
	}

	for _, conv := range conversations {
		summary := ConversationSummary{
			ID:        conv.ID,
			Title:     GenerateTitle(conv),
			UserHash:  conv.User.ID,
			CreatedAt: conv.CreatedAt,
			Turns:     conv.Stats.Turns,
			CostUSD:   conv.Stats.TotalCostUSD,
		}

		if idx, ok := existingMap[conv.ID]; ok {
			existing.Conversations[idx] = summary
		} else {
			existing.Conversations = append(existing.Conversations, summary)
		}
	}

	// Recalculate totals
	existing.TotalConversations = len(existing.Conversations)
	totalMessages := 0
	totalCost := 0.0
	for _, c := range existing.Conversations {
		totalMessages += c.Turns
		totalCost += c.CostUSD
	}
	existing.TotalMessages = totalMessages
	existing.TotalCostUSD = totalCost

	return existing
}

// ShouldCleanup returns true if a conversation is older than the retention period.
func ShouldCleanup(createdAt time.Time, retentionDays int) bool {
	if retentionDays <= 0 {
		return false
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	return createdAt.Before(cutoff)
}

// NewConversation creates a new conversation with the given parameters.
func NewConversation(agentFile, model, userID, displayName string) *Conversation {
	now := time.Now().UTC()
	return &Conversation{
		ID:          GenerateConversationID(),
		CreatedAt:   now,
		UpdatedAt:   now,
		User:        ConversationUser{ID: userID, DisplayName: displayName},
		AgentConfig: agentFile,
		Model:       model,
		Stats:       ConversationStats{},
		Messages:    make([]Message, 0),
	}
}

// AddMessage appends a message to the conversation and updates stats.
func (c *Conversation) AddMessage(msg Message) {
	c.Messages = append(c.Messages, msg)
	c.UpdatedAt = time.Now().UTC()

	if msg.Role == "user" || msg.Role == "assistant" {
		c.Stats.Turns = len(c.Messages)
	}

	if msg.Usage != nil {
		c.Stats.TotalInputTokens += msg.Usage.InputTokens
		c.Stats.TotalOutputTokens += msg.Usage.OutputTokens
		c.Stats.TotalCostUSD += msg.Usage.CostUSD
	}

	for _, tc := range msg.ToolCalls {
		c.Stats.ToolsCalled = append(c.Stats.ToolsCalled, tc.Tool)
	}
}
