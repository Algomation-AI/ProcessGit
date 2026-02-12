// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateConversationID(t *testing.T) {
	id1 := GenerateConversationID()
	id2 := GenerateConversationID()
	assert.True(t, len(id1) > 5)
	assert.Contains(t, id1, "conv_")
	assert.NotEqual(t, id1, id2)
}

func TestConversationFilePath(t *testing.T) {
	conv := &Conversation{
		ID:        "conv_abcd1234",
		CreatedAt: time.Date(2026, 2, 11, 14, 30, 0, 0, time.UTC),
	}
	path := ConversationFilePath(conv)
	assert.Equal(t, "2026/02/11/conv_abcd1234.json", path)
}

func TestGenerateTitle(t *testing.T) {
	t.Run("NormalMessage", func(t *testing.T) {
		conv := &Conversation{
			Messages: []Message{
				{Role: "user", Content: "Where to classify GDPR letter?"},
			},
		}
		title := GenerateTitle(conv)
		assert.Equal(t, "Where to classify GDPR letter?", title)
	})

	t.Run("LongMessage", func(t *testing.T) {
		conv := &Conversation{
			Messages: []Message{
				{Role: "user", Content: "This is a very long message that exceeds the maximum title length and should be truncated properly"},
			},
		}
		title := GenerateTitle(conv)
		assert.True(t, len(title) <= maxTitleLength+3) // +3 for "..."
		assert.Contains(t, title, "...")
	})

	t.Run("NoUserMessage", func(t *testing.T) {
		conv := &Conversation{
			Messages: []Message{
				{Role: "assistant", Content: "Welcome!"},
			},
		}
		title := GenerateTitle(conv)
		assert.Equal(t, "New conversation", title)
	})

	t.Run("MultilineMessage", func(t *testing.T) {
		conv := &Conversation{
			Messages: []Message{
				{Role: "user", Content: "First line\nSecond line"},
			},
		}
		title := GenerateTitle(conv)
		assert.NotContains(t, title, "\n")
	})
}

func TestBuildUpdatedIndex(t *testing.T) {
	t.Run("NewIndex", func(t *testing.T) {
		convs := []*Conversation{
			{
				ID:        "conv_001",
				CreatedAt: time.Now(),
				User:      ConversationUser{ID: "user1"},
				Stats:     ConversationStats{Turns: 4, TotalCostUSD: 0.05},
				Messages:  []Message{{Role: "user", Content: "Hello"}},
			},
		}
		index := BuildUpdatedIndex(nil, convs)
		assert.Equal(t, "1.0", index.Version)
		assert.Equal(t, 1, index.TotalConversations)
		assert.Equal(t, 4, index.TotalMessages)
		assert.InDelta(t, 0.05, index.TotalCostUSD, 0.001)
		assert.Equal(t, "Hello", index.Conversations[0].Title)
	})

	t.Run("UpdateExisting", func(t *testing.T) {
		existing := &ConversationIndex{
			Version: "1.0",
			Conversations: []ConversationSummary{
				{ID: "conv_001", Turns: 2, CostUSD: 0.02},
			},
		}
		convs := []*Conversation{
			{
				ID:        "conv_001",
				CreatedAt: time.Now(),
				User:      ConversationUser{ID: "user1"},
				Stats:     ConversationStats{Turns: 6, TotalCostUSD: 0.08},
				Messages:  []Message{{Role: "user", Content: "Updated"}},
			},
		}
		index := BuildUpdatedIndex(existing, convs)
		assert.Equal(t, 1, index.TotalConversations) // no duplicate
		assert.Equal(t, 6, index.Conversations[0].Turns)
	})

	t.Run("AddNewToExisting", func(t *testing.T) {
		existing := &ConversationIndex{
			Version: "1.0",
			Conversations: []ConversationSummary{
				{ID: "conv_001", Turns: 2, CostUSD: 0.02},
			},
		}
		convs := []*Conversation{
			{
				ID:        "conv_002",
				CreatedAt: time.Now(),
				User:      ConversationUser{ID: "user2"},
				Stats:     ConversationStats{Turns: 3, TotalCostUSD: 0.03},
				Messages:  []Message{{Role: "user", Content: "New conv"}},
			},
		}
		index := BuildUpdatedIndex(existing, convs)
		assert.Equal(t, 2, index.TotalConversations)
	})
}

func TestShouldCleanup(t *testing.T) {
	t.Run("OldConversation", func(t *testing.T) {
		old := time.Now().AddDate(0, 0, -100)
		assert.True(t, ShouldCleanup(old, 90))
	})

	t.Run("RecentConversation", func(t *testing.T) {
		recent := time.Now().AddDate(0, 0, -10)
		assert.False(t, ShouldCleanup(recent, 90))
	})

	t.Run("ZeroRetention", func(t *testing.T) {
		old := time.Now().AddDate(0, 0, -1000)
		assert.False(t, ShouldCleanup(old, 0))
	})
}

func TestNewConversation(t *testing.T) {
	conv := NewConversation("agent.chat.yaml", "claude-sonnet-4-5", "user123", "Test User")
	assert.Contains(t, conv.ID, "conv_")
	assert.Equal(t, "agent.chat.yaml", conv.AgentConfig)
	assert.Equal(t, "claude-sonnet-4-5", conv.Model)
	assert.Equal(t, "user123", conv.User.ID)
	assert.Equal(t, "Test User", conv.User.DisplayName)
	assert.NotZero(t, conv.CreatedAt)
}

func TestAddMessage(t *testing.T) {
	conv := NewConversation("agent.chat.yaml", "claude-sonnet-4-5", "u1", "User")

	msg := Message{
		Role:    "user",
		Content: "Hello",
		Timestamp: time.Now(),
	}
	conv.AddMessage(msg)
	assert.Equal(t, 1, len(conv.Messages))
	assert.Equal(t, 1, conv.Stats.Turns)

	assistantMsg := Message{
		Role:    "assistant",
		Content: "Hi there!",
		Timestamp: time.Now(),
		Usage:   &Usage{InputTokens: 100, OutputTokens: 50, CostUSD: 0.01},
		ToolCalls: []ToolCall{{Tool: "search", Server: "test"}},
	}
	conv.AddMessage(assistantMsg)
	assert.Equal(t, 2, len(conv.Messages))
	assert.Equal(t, 100, conv.Stats.TotalInputTokens)
	assert.Equal(t, 50, conv.Stats.TotalOutputTokens)
	assert.InDelta(t, 0.01, conv.Stats.TotalCostUSD, 0.001)
	assert.Equal(t, []string{"search"}, conv.Stats.ToolsCalled)
}

func TestConversationBuffer(t *testing.T) {
	buf := &ConversationBuffer{
		conversations: make(map[string]*Conversation),
		lastFlush:     time.Now(),
	}

	// Empty buffer should not flush
	assert.False(t, buf.ShouldFlush())

	// Add conversations
	for i := 0; i < batchFlushThreshold; i++ {
		conv := NewConversation("agent.chat.yaml", "model", "user", "User")
		buf.BufferConversation(conv)
	}

	// Should flush when threshold is reached
	assert.True(t, buf.ShouldFlush())

	// Drain and verify
	drained := buf.DrainConversations()
	assert.Equal(t, batchFlushThreshold, len(drained))

	// Buffer should be empty after drain
	assert.False(t, buf.ShouldFlush())
	assert.Empty(t, buf.DrainConversations())
}
