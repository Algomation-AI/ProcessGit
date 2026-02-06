// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

// MCP server settings
var MCP = struct {
	Enabled            bool
	MaxServersPerUser  int
	RateLimitPerMinute int
	SessionTimeoutSec  int
	MaxResponseSizeMB  int
}{
	Enabled:            true,
	MaxServersPerUser:  50,
	RateLimitPerMinute: 120,
	SessionTimeoutSec:  3600,
	MaxResponseSizeMB:  5,
}

func loadMCPFrom(rootCfg ConfigProvider) {
	sec := rootCfg.Section("mcp")
	MCP.Enabled = sec.Key("ENABLED").MustBool(true)
	MCP.MaxServersPerUser = sec.Key("MAX_SERVERS_PER_USER").MustInt(50)
	MCP.RateLimitPerMinute = sec.Key("RATE_LIMIT_PER_MINUTE").MustInt(120)
	MCP.SessionTimeoutSec = sec.Key("SESSION_TIMEOUT").MustInt(3600)
	MCP.MaxResponseSizeMB = sec.Key("MAX_RESPONSE_SIZE_MB").MustInt(5)
}
