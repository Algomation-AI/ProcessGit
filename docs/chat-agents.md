# ProcessGit Chat Agents

Chat agents provide an interactive AI chat interface within ProcessGit repositories. When a repository contains an `agent.chat.yaml` file, ProcessGit renders a chat panel instead of raw YAML when the file is clicked in the file tree.

## Quick Start

1. Create an `agent.chat.yaml` file in your repository root:

```yaml
version: "1.0"

ui:
  name: "My Assistant"
  subtitle: "Ask me anything about this project"
  welcome_message: |
    Hello! I can help you understand this repository.
  quick_questions:
    - "What does this project do?"
    - "How is the data structured?"

llm:
  provider: "anthropic"
  model: "claude-sonnet-4-5"
  api_key_ref: "ANTHROPIC_API_KEY"
  max_tokens: 1500
  temperature: 0.3
  system_prompt: |
    You are a helpful assistant for this repository.
    Use the available MCP tools to search and retrieve data.

mcp:
  use_repo_mcp: true
```

2. Ensure your ProcessGit instance has the `ANTHROPIC_API_KEY` environment variable set.

3. Navigate to your repository in ProcessGit — the file tree will show the agent with a robot icon and its friendly name.

4. Click the agent entry to open the chat panel.

## File Discovery

ProcessGit scans repositories for chat agent configurations using these priority rules:

| Priority | Path | Description |
|----------|------|-------------|
| 1 | `agent.chat.yaml` | Root directory (default) |
| 2 | `.processgit/agent.chat.yaml` | Config directory |
| 3 | `*.agent.chat.yaml` | Named variants (e.g., `classification.agent.chat.yaml`) |

Multiple `*.agent.chat.yaml` files in one repository create multiple chat agents, each shown as a separate clickable item in the file tree.

## Full YAML Reference

### `version` (required)

Config format version. Currently `"1.0"`.

### `ui` — User Interface Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | **yes** | — | Chat window title |
| `subtitle` | string | no | — | Subtitle under title |
| `icon` | string | no | `"🤖"` | Emoji or icon |
| `language` | string | no | `"en"` | Primary UI language |
| `placeholder` | string | no | `"Ask a question..."` | Input placeholder |
| `welcome_message` | string | no | — | Initial assistant message |
| `quick_questions` | string[] | no | — | Preset question bubbles |

#### `ui.theme` — Theme Customization

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `primary_color` | string | `"#1a5276"` | Header/accent color |
| `assistant_avatar` | string | `"🤖"` | Avatar for assistant messages |
| `user_avatar` | string | `"👤"` | Avatar for user messages |
| `max_height` | string | `"600px"` | Chat panel max height |

### `llm` — Language Model Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `provider` | string | **yes** | — | `"anthropic"`, `"openai"`, or `"ollama"` |
| `model` | string | **yes** | — | Model identifier (e.g., `"claude-sonnet-4-5"`) |
| `api_key_ref` | string | **yes** | — | Environment variable name for API key |
| `max_tokens` | int | no | `1024` | Maximum response tokens |
| `temperature` | float | no | `0.3` | Sampling temperature (lower = more factual) |
| `top_p` | float | no | `0.9` | Nucleus sampling threshold |
| `system_prompt` | string | no | — | System prompt defining assistant behavior |

### `mcp` — MCP Tool Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `use_repo_mcp` | bool | `false` | Use this repo's own MCP server |
| `additional_servers` | array | — | Extra MCP servers for cross-repo queries |
| `allowed_tools` | string[] | — | Only these tools are available (whitelist) |
| `denied_tools` | string[] | — | These tools are blocked (blacklist) |

Each entry in `additional_servers`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Server identifier |
| `url` | string | MCP server URL |
| `description` | string | Human-readable description |

### `history` — Conversation Persistence

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable conversation storage |
| `storage` | string | `"git-branch"` | Storage backend |
| `branch` | string | `"chat-history"` | Git branch for conversations |
| `retention_days` | int | `90` | Auto-cleanup after N days |
| `max_conversations_per_user` | int | `100` | Per-user conversation limit |
| `anonymize` | bool | `false` | Strip user identifiers |

### `access` — Rate Limiting & Access Control

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `visibility` | string | `"authenticated"` | `"public"`, `"authenticated"`, or `"team"` |

#### `access.rate_limits`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `requests_per_minute` | int | `10` | Per-user rate limit |
| `requests_per_day` | int | `100` | Per-user daily limit |
| `max_conversation_turns` | int | `50` | Max messages per conversation |

#### `access.budget`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_monthly_usd` | float | — | Stop serving when exceeded |
| `alert_threshold_pct` | int | `80` | Alert admin at this percentage |

## API Key Management

API keys are referenced by environment variable name — **never store actual keys in `agent.chat.yaml`**.

```yaml
llm:
  api_key_ref: "ANTHROPIC_API_KEY"  # env var name on the ProcessGit server
```

Set the environment variable on your ProcessGit server:

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Chat History

When `history.enabled: true`, conversations are stored on an orphan git branch (default: `chat-history`). This provides:

- Immutable audit trail via git commit history
- Portability — clone includes conversation history
- No additional infrastructure required

### Branch Structure

```
chat-history branch:
├── _index.json                    # Conversation index
├── 2026/
│   └── 02/
│       └── 11/
│           ├── conv_a1b2c3d4.json
│           └── conv_e5f6g7h8.json
└── _stats/
    └── 2026-02.json               # Monthly usage statistics
```

Conversations are batched — commits happen every 5 minutes or when 10+ conversations are updated, to avoid polluting git history.

## Example Configurations

### Minimal Configuration

```yaml
version: "1.0"
ui:
  name: "Project Helper"
llm:
  provider: "anthropic"
  model: "claude-sonnet-4-5"
  api_key_ref: "ANTHROPIC_API_KEY"
```

### Full Configuration with MCP Tools

```yaml
version: "1.0"
ui:
  name: "Classification Assistant"
  subtitle: "Document classification help"
  icon: "📋"
  language: "lv"
  placeholder: "Ask about classification..."
  welcome_message: |
    Welcome! I can help you classify documents.
  quick_questions:
    - "What are the main categories?"
    - "Where to classify a GDPR letter?"
  theme:
    primary_color: "#1a5276"

llm:
  provider: "anthropic"
  model: "claude-sonnet-4-5"
  api_key_ref: "ANTHROPIC_API_KEY"
  max_tokens: 1500
  temperature: 0.3
  system_prompt: |
    You are a classification assistant. Use MCP tools to
    search and retrieve classification data. Always cite
    specific category codes in bold.

mcp:
  use_repo_mcp: true
  additional_servers:
    - name: "org-register"
      url: "https://processgit.org/VARAM/Organizations/mcp"
      description: "Organization register"
  allowed_tools:
    - search
    - get_entity
    - describe_model

history:
  enabled: true
  retention_days: 90

access:
  visibility: "authenticated"
  rate_limits:
    requests_per_minute: 10
    requests_per_day: 100
  budget:
    max_monthly_usd: 50.00
```

## ProcessGit Server Configuration

Chat agents can be configured globally in the ProcessGit configuration file (`app.ini`):

```ini
[chat]
; Enable/disable chat agents globally
ENABLED = true
; Maximum chat agents per repository
MAX_AGENTS_PER_REPO = 10
; Global rate limit per minute
RATE_LIMIT_PER_MINUTE = 10
; Maximum monthly budget across all repos
MAX_MONTHLY_BUDGET = 100.0
; Default LLM provider
DEFAULT_PROVIDER = anthropic
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/{owner}/{repo}/chat` | Send a message |
| `GET` | `/{owner}/{repo}/chat/agents` | List chat agents |
| `GET` | `/{owner}/{repo}/chat/history` | List conversations |

### POST `/{owner}/{repo}/chat`

Request body:
```json
{
  "message": "Your question here",
  "conversation_id": "conv_abc123",
  "agent_file": "agent.chat.yaml"
}
```

Response: Server-Sent Events stream with events:
- `message_delta` — text chunk: `{"type": "text", "text": "..."}`
- `tool_use` — tool call: `{"type": "tool_call", "tool": "search", "server": "..."}`
- `message_complete` — done: `{"type": "done", "conversation_id": "...", "usage": {...}}`

## Troubleshooting

**Chat panel doesn't appear**: Verify `agent.chat.yaml` is in the repository root or `.processgit/` directory. Check that `[chat] ENABLED = true` in the server configuration.

**API key errors**: Ensure the environment variable referenced by `api_key_ref` is set on the ProcessGit server.

**Rate limit errors (429)**: The user has exceeded the configured rate limits. Wait and retry, or increase limits in the YAML config.

**Budget exceeded (402)**: Monthly budget has been reached. Increase `access.budget.max_monthly_usd` or wait for the next billing month.

**MCP tools not working**: Ensure the repository has a valid `processgit.mcp.yaml` and that `mcp.use_repo_mcp: true` is set. Verify MCP is enabled globally (`[mcp] ENABLED = true`).
