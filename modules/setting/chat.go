// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// Chat agent settings
var Chat = struct {
	Enabled            bool
	MaxAgentsPerRepo   int
	RateLimitPerMinute int
	MaxMonthlyBudget   float64
	DefaultProvider    string
}{
	Enabled:            true,
	MaxAgentsPerRepo:   10,
	RateLimitPerMinute: 10,
	MaxMonthlyBudget:   100.0,
	DefaultProvider:    "anthropic",
}

func loadChatFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("chat")
	Chat.Enabled = sec.Key("ENABLED").MustBool(true)
	Chat.MaxAgentsPerRepo = sec.Key("MAX_AGENTS_PER_REPO").MustInt(10)
	Chat.RateLimitPerMinute = sec.Key("RATE_LIMIT_PER_MINUTE").MustInt(10)
	Chat.MaxMonthlyBudget = sec.Key("MAX_MONTHLY_BUDGET").MustFloat64(100.0)
	Chat.DefaultProvider = sec.Key("DEFAULT_PROVIDER").MustString("anthropic")
}
