# ProcessGit

[![GitHub tag](https://img.shields.io/github/v/tag/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/tags)
[![GitHub license](https://img.shields.io/github/license/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/blob/main/LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/issues)
[![GitHub stars](https://img.shields.io/github/stars/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/stargazers)

**ProcessGit** is a Git-based repository platform for **executable processes and algorithms**. It is a fork of [Gitea](https://github.com/go-gitea/gitea) (MIT licensed), extended with native support for process modeling standards, visual diagram editing, UAPF packaging, and a custom viewer/editor framework that lets repository authors ship their own HTML-based GUIs alongside data files.

**Public demo / test instance:** https://processgit.org

> **Important notice**
> The public instance is fully functional but intended for **testing, evaluation, and demonstration only**. Data may be reset at any time. No availability or durability guarantees.

---

## What is ProcessGit

ProcessGit is a **Process Repository** (sometimes called a *processpository*). It brings version control, review, releases, and traceability — the proven disciplines of source-code management — to **process logic**, workflows, and algorithmic definitions that traditionally live in documents, diagrams, or proprietary tools.

ProcessGit allows you to:

- Store executable processes as versioned artifacts in Git
- View and edit BPMN, CMMN, and DMN diagrams directly in the browser
- Visualize N-Graph networks, Rulesets, XSD schemas, and XML classifications with built-in viewers
- Import and export UAPF algorithm packages with schema validation
- Ship custom HTML viewers/editors alongside any data file using a simple manifest
- Expose repository data to AI agents via built-in MCP (Model Context Protocol) servers
- Embed AI chat assistants directly in repositories with configurable LLM providers
- Classify repositories by type (process, decision, reference, connector) and lifecycle status
- Tag, release, and review process definitions through commits and pull requests
- Treat organizational workflows as **first-class governed assets**

Instead of managing only source code, ProcessGit manages **how work is done**.

---

## Core Features

### 1. Built-in File Viewers & Editors

ProcessGit ships with a comprehensive set of viewers and editors that automatically activate when you navigate to a supported file. Instead of showing raw source, the platform renders an interactive graphical interface.

#### Complete Viewer Inventory

| # | Viewer | File Extensions | Format | Capabilities | Library / Technology |
|---|--------|----------------|--------|-------------|---------------------|
| 1 | **BPMN 2.0** — Business Process Model and Notation | `.bpmn`, `.bpmn20.xml`, `*bpmn.xml` | XML | View + Edit + Properties panel | [bpmn-js](https://bpmn.io/), bpmn-auto-layout |
| 2 | **CMMN 1.1** — Case Management Model and Notation | `.cmmn`, `.cmmn11.xml`, `*cmmn.xml` | XML | View + Edit + Properties panel | cmmn-js |
| 3 | **DMN 1.3** — Decision Model and Notation | `.dmn`, `.dmn11.xml`, `*dmn.xml` | XML | View + Edit + Properties panel (DRD, decision table, literal expression) | dmn-js |
| 4 | **N-Graph** — Network / Graph visualization | `.ngraph.json`, `.ngraph.xml`, `.ngraph` | JSON / XML | View only | [Cytoscape.js](https://js.cytoscape.org/) |
| 5 | **Ruleset** — Business rules | `.ruleset.json`, `.ruleset.dmn`, `.ruleset` | JSON / DMN | View only (searchable table: Name, When, Then, Priority) | Custom HTML table |
| 6 | **XML Classification** — Structured XML viewer for schemas and registers | Detected by XML content sniffing | XML | Preview + Edit + Raw | Custom (classification & document metadata renderers) |
| 7 | **XSD Visual** — XML Schema Definition | `.xsd` | XML (XSD) | Visual graph editor (elements, complex types, relationships) | Custom SVG rendering with parse/serialize |
| 8 | **ProcessGit Custom Viewer** — User-defined HTML GUIs | Matched by `processgit.viewer.json` manifest | HTML | Fully custom (sandboxed iframe) | PGV postMessage protocol |

#### Detection & Rendering

**File detection** works by extension first, falling back to content sniffing (e.g. checking for `xmlns:bpmn=` in XML, or domain-specific root elements). BPMN files missing diagram layout information (`bpmndi:BPMNDiagram`) are auto-laid-out before rendering.

**Three-mode toolbar** — diagram files get a toolbar with *Preview*, *Edit*, and *Raw* tabs. Preview renders the graphical diagram (read-only, fitted to viewport). Edit opens a full modeler with a properties panel for BPMN/CMMN/DMN. Raw shows the underlying XML/JSON source. A **Save** button commits changes directly to the repository branch.

**Template repositories** — ProcessGit bootstraps starter templates (BPMN process, DMN decision, CMMN case, UAPF variants) that appear in the "New Repository" template dropdown.

---

### 2. UAPF Package Support (Unified Algorithmic Process Format)

UAPF is a packaging standard for bundling process artifacts — workflows, decision models, governance metadata — into a single portable `.uapf` archive (ZIP-based) with a `manifest.json` at its root.

**Import:** Upload a `.uapf` file through the repository UI (via the import modal). The package is validated against an embedded JSON Schema (`uapf-manifest.schema.json`, Draft 2020-12), extracted safely, and committed into the repository. Referenced file paths in the manifest are verified to exist in the archive. Conflicts with existing repository files are detected and rejected.

**Export:** Download the current repository contents (at any ref/branch) as a `.uapf` archive. The export validates the `manifest.json`, resolves all referenced paths, and streams a ZIP file named `{package}_{version}.uapf`.

**Manifest validation** — both import and export validate the manifest structure, including `name`, `version`, `package` metadata, and arrays of `workflows` and `resources`, each referencing internal file paths with a declared type.

**UAPF levels** — repositories can be classified with a UAPF level (0–4) in the platform metadata, corresponding to organizational hierarchy depth (L0 = enterprise, L4 = task-level).

**API routes:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/{owner}/{repo}/uapf/import` | Upload and import a `.uapf` package |
| `GET` | `/{owner}/{repo}/uapf/export?ref=` | Download repo as `.uapf` package |

---

### 3. Custom Viewers & Editors (`processgit.viewer.json`)

This is one of ProcessGit's most distinctive capabilities. Any repository can ship a **custom HTML-based viewer/editor** for its data files. When a user navigates to a matched file, ProcessGit loads the custom HTML GUI in a sandboxed iframe in the right pane instead of showing raw source.

This means domain experts can build tailored editing experiences — a register editor for XML data, a form-based configuration tool, a visual schema browser — and distribute them alongside the data, all within the same Git repository.

#### How It Works

1. Place a `processgit.viewer.json` manifest in any directory of your repository
2. Place your HTML viewer file(s) in the same directory
3. When a user browses to a file matching a viewer's `primary_pattern`, ProcessGit renders the custom HTML viewer instead of raw source

The platform provides a two-tab interface: **GUI** (the custom viewer) and **Raw** (the underlying source). A **Save** button is enabled when the viewer reports unsaved changes.

#### Manifest Format (`processgit.viewer.json`)

```json
{
  "version": 1,
  "viewers": [
    {
      "id": "my-viewer",
      "primary_pattern": "data-file.xml",
      "type": "html",
      "entry": "viewer.html",
      "edit_allow": ["data-file.xml"],
      "targets": {
        "xsd": "schema.xsd",
        "xml": "data-file.xml"
      }
    }
  ]
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `version` | Yes | Manifest version (currently `1`) |
| `viewers` | Yes | Array of viewer bindings (at least one) |
| `viewers[].id` | Yes | Unique identifier for this viewer |
| `viewers[].primary_pattern` | Yes | Glob pattern to match the target file (Go `path.Match` semantics). Examples: `"data.xml"`, `"*-register.xml"`, `"registers/*.xml"` |
| `viewers[].type` | Yes | Must be `"html"` (v1 only supports HTML viewers) |
| `viewers[].entry` | Yes | Path to the HTML file relative to the manifest directory |
| `viewers[].edit_allow` | Yes | List of file paths the viewer is permitted to save back |
| `viewers[].targets` | No | Key-value map of related files the viewer may need (schemas, examples, etc.). Values are paths relative to the manifest directory |

#### Building a Custom Viewer

Your viewer HTML runs inside a **sandboxed iframe** (`allow-scripts allow-forms`). It communicates with ProcessGit through the **PGV (ProcessGit Viewer) postMessage protocol**:

**Lifecycle:**

1. **Viewer loads** → Send `PGV_READY` to parent
2. **Host responds** → Sends `PGV_INIT` with a payload containing repo context, file paths, target URLs, and edit permissions
3. **Viewer fetches data** → Use `PGV_FETCH` to request file contents through the host's same-origin proxy (direct fetch from the iframe sandbox won't work for repo files)
4. **User edits data** → Send `PGV_DIRTY` with `{ dirty: true }` to enable the Save button
5. **User saves** → Respond to `PGV_SAVE_CLICKED` by sending `PGV_REQUEST_SAVE` with the updated content

**Message Reference:**

| Direction | Message Type | Payload | Purpose |
|-----------|-------------|---------|---------|
| Viewer → Host | `PGV_READY` | — | Signal the viewer is loaded and ready |
| Host → Viewer | `PGV_INIT` | `{ payload: ProcessGitViewerPayload }` | Deliver repo context, file paths, targets |
| Viewer → Host | `PGV_FETCH` | `{ url, reqId }` | Request a file through the host's same-origin proxy |
| Host → Viewer | `PGV_FETCH_RESULT` | `{ reqId, url, ok, text?, error? }` | Response to a fetch request |
| Viewer → Host | `PGV_DIRTY` | `{ dirty: boolean }` | Toggle the Save button state |
| Viewer → Host | `PGV_SET_CONTENT` | `{ path, content }` | Stage content for saving |
| Host → Viewer | `PGV_SAVE_CLICKED` | — | User clicked Save; viewer should trigger save |
| Viewer → Host | `PGV_REQUEST_SAVE` | `{ path, content, summary? }` | Commit the content to the repository |
| Host → Viewer | `PGV_SAVE_RESULT` | `{ ok, error? }` | Result of the save operation |
| Viewer → Host | `PGV_REQUEST_LOAD` | `{ path }` | Request content of a specific target file |
| Host → Viewer | `PGV_LOAD_RESULT` | `{ path, content }` | Response with file content |

**Minimal viewer template:**

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>My Custom Viewer</title>
</head>
<body>
  <div id="app">Loading...</div>
  <script>
    let payload = null;

    // Helper: fetch a file through the host proxy
    function pgvFetch(url) {
      return new Promise((resolve, reject) => {
        const reqId = 'req-' + Date.now() + '-' + Math.random().toString(16).slice(2);
        const timeout = setTimeout(() => reject(new Error('PGV_FETCH timeout')), 15000);

        function onMsg(ev) {
          const m = ev.data;
          if (!m || m.type !== 'PGV_FETCH_RESULT' || m.reqId !== reqId) return;
          window.removeEventListener('message', onMsg);
          clearTimeout(timeout);
          m.ok ? resolve(m.text) : reject(new Error(m.error));
        }

        window.addEventListener('message', onMsg);
        parent.postMessage({ type: 'PGV_FETCH', url, reqId }, '*');
      });
    }

    // Listen for host messages
    window.addEventListener('message', async (ev) => {
      const msg = ev.data;
      if (!msg) return;

      if (msg.type === 'PGV_INIT') {
        payload = msg.payload;

        // Fetch the primary file content via targets
        const xmlUrl = payload.targets.xml || payload.entryRawUrl;
        const content = await pgvFetch(xmlUrl);

        // Render your UI
        document.getElementById('app').textContent = content;
      }

      if (msg.type === 'PGV_SAVE_CLICKED') {
        const editedContent = getEditedContent(); // your logic
        parent.postMessage({
          type: 'PGV_REQUEST_SAVE',
          path: payload.path,
          content: editedContent,
          summary: 'Update via custom viewer'
        }, '*');
      }
    });

    // Signal readiness
    parent.postMessage({ type: 'PGV_READY' }, '*');
  </script>
</body>
</html>
```

**Security model:** The iframe is sandboxed. File fetches are proxied through the host using a strict allow-list: only same-origin URLs matching `/raw/` or `/src/` paths are permitted. The `edit_allow` array in the manifest controls which files the viewer can write back.

#### Real-World Example: Organization Register Viewer

A custom viewer for an XML register of organizations, validated against an XSD schema.

**Repository structure:**
```
processgit.viewer.json          ← Manifest
register-viewer.html            ← Custom HTML viewer/editor
register.xml                    ← Data file (the register)
register.xsd                    ← XSD schema for validation
```

**Manifest:**
```json
{
  "version": 1,
  "viewers": [
    {
      "id": "org-register",
      "primary_pattern": "register.xml",
      "type": "html",
      "entry": "register-viewer.html",
      "edit_allow": ["register.xml"],
      "targets": {
        "xsd": "register.xsd",
        "xml": "register.xml"
      }
    }
  ]
}
```

When a user browses to `register.xml`, instead of seeing raw XML, they get a fully interactive table editor with search, sorting, inline editing, XSD validation status, and the ability to save changes back to the repository — all from a single self-contained HTML file shipped in the same repo.

---

### 4. Custom Viewers To Be Made

The ProcessGit custom viewer framework (`processgit.viewer.json`) makes it straightforward to build new domain-specific viewers. The following are planned custom viewers to be developed and shipped as reference implementations:

| Viewer | Target Files | Description | Status |
|--------|-------------|-------------|--------|
| **JSON Schema Viewer** | `.schema.json`, `.json-schema` | Visual form-based editor for JSON Schema definitions with property tree, type selectors, and validation preview | Planned |
| **OpenAPI / Swagger Viewer** | `.openapi.yaml`, `.openapi.json`, `.swagger.json` | Interactive API documentation viewer with try-it-out request builder and schema visualization | Planned |
| **Markdown Rich Editor** | `.md`, `.markdown` | WYSIWYG markdown editor with live preview, table editor, and embedded diagram support | Planned |
| **CSV / Spreadsheet Viewer** | `.csv`, `.tsv`, `.xlsx` | Tabular data viewer with sorting, filtering, column statistics, and cell-level editing | Planned |
| **YAML Config Editor** | `.yaml`, `.yml` (with schema) | Schema-aware YAML editor with autocomplete, validation, and visual form rendering | Planned |
| **Gantt / Timeline Viewer** | `.gantt.json`, `.timeline.json` | Interactive Gantt chart for project scheduling and process timeline visualization | Planned |
| **Form Builder** | `.form.json`, `.form.yaml` | Drag-and-drop form designer for building data entry forms tied to repository schemas | Planned |
| **Process KPI Dashboard** | `.kpi.json`, `.metrics.yaml` | Dashboard viewer for process metrics, KPIs, and SLA definitions with chart rendering | Planned |
| **Data Flow Diagram Viewer** | `.dfd.json`, `.dataflow.xml` | Visual editor for data flow diagrams showing data stores, processes, and flows | Planned |
| **Regulation / Policy Viewer** | `.regulation.xml`, `.policy.json` | Structured viewer for legal and regulatory documents with cross-reference navigation | Planned |

#### How to Build Your Own Custom Viewer

Any repository can ship a custom HTML-based viewer. See the [Custom Viewers & Editors](#3-custom-viewers--editors-processgitviewerjson) section for the full `processgit.viewer.json` manifest format and PGV postMessage protocol reference.

Key steps:
1. Create a `processgit.viewer.json` manifest defining your viewer's `id`, `primary_pattern`, `entry` HTML file, and `edit_allow` list
2. Build your viewer HTML using the PGV protocol (`PGV_READY` → `PGV_INIT` → `PGV_FETCH` → `PGV_DIRTY` → `PGV_REQUEST_SAVE`)
3. Place both files in the same repository directory
4. Navigate to a matching file — ProcessGit renders your viewer instead of raw source

Community contributions of new viewers are welcome. Open an issue to discuss your viewer idea before submitting a pull request.

---

### 5. Repository Classification

ProcessGit tracks repository classification directly in the platform database. Each repository has metadata fields that categorize its purpose and lifecycle stage.

| Field | Values | Description |
|-------|--------|-------------|
| `repo_type` | `process`, `decision`, `reference`, `connector`, `template` | What kind of artifact the repo stores |
| `uapf_level` | `0`–`4` or null | UAPF hierarchy level (L0=enterprise … L4=task) |
| `reference_kind` | `schema`, `classifier`, `register`, `codelist`, `vocabulary`, `standard` | Sub-classification for reference repos |
| `status` | `draft`, `stable`, `deprecated`, `archived` | Lifecycle status |

A default classification (`repo_type=process`, `status=draft`) is created automatically when a repository is created.

---

## Typical Use Cases

- Executable business processes (BPMN-based workflows with visual editing)
- Decision logic and rulesets (DMN decision tables and graphs)
- Case management models (CMMN cases)
- Algorithm packages (`.uapf` bundles with schema-validated manifests)
- Public-sector and enterprise process catalogs with governed releases
- Reference data registers with custom HTML viewers/editors
- XSD schemas with visual browsing and editing
- Network/graph data visualization (N-Graph with Cytoscape.js)
- Document classification schemes (XML structured preview)
- AI-powered data assistants for repository content (chat agents with MCP tools)
- Exposing structured data to external AI agents via MCP server endpoints
- Cross-repository AI queries connecting multiple MCP servers
- AI-assisted execution engines that require governed, versioned inputs
- Single Source of Truth (SSOT) for operational logic

---

## MCP Server Integration (Model Context Protocol)

ProcessGit implements a built-in **MCP server** for every repository, enabling AI agents and LLMs to query, search, and validate repository data through a standardized protocol. This turns every ProcessGit repository into an AI-accessible knowledge base.

### How It Works

Any repository containing a `processgit.mcp.yaml` configuration file exposes an MCP server endpoint. External AI tools (Claude Desktop, custom agents, other MCP clients) can connect to this endpoint and interact with the repository data using structured tool calls.

**Protocol:** JSON-RPC 2.0 over HTTP with Server-Sent Events (SSE) streaming.

**Endpoint:** `GET/POST /{owner}/{repo}/mcp`

### MCP Configuration (`processgit.mcp.yaml`)

```yaml
version: 1

server:
  name: "my-data-server"
  description: "MCP server for organization register data"
  instructions: |
    This server provides access to organizational data.
    Use 'search' to find entities and 'get_entity' for details.

sources:
  - path: "data/organizations.xml"
    type: "xml"
    schema: "data/organizations.xsd"
    description: "Registry of organizations"
  - path: "data/classifications.xml"
    type: "xml"
    description: "Document classification scheme"
```

| Field | Required | Description |
|-------|----------|-------------|
| `version` | Yes | Config version (currently `1`) |
| `server.name` | Yes | Human-readable server name |
| `server.description` | No | Server purpose description |
| `server.instructions` | No | Usage instructions for AI agents |
| `sources` | Yes | Array of data sources (at least 1) |
| `sources[].path` | Yes | Path to the data file in the repo |
| `sources[].type` | Yes | Data type (`xml` currently supported) |
| `sources[].schema` | No | Path to XSD/JSON Schema for validation |
| `sources[].description` | No | Human-readable description of the source |

### Available MCP Tools

When an AI agent connects to a ProcessGit MCP server, it has access to these tools:

| Tool | Description |
|------|-------------|
| `help` | Returns server capabilities and usage instructions |
| `identify` | Returns server identity, repository info, and available sources |
| `describe_model` | Describes the data model, entity types, and their attributes |
| `search` | Full-text search across all indexed entities |
| `get_entity` | Retrieve a specific entity by ID or path |
| `list_entities` | List all entities with optional filtering |
| `validate` | Validate data against its XML/JSON schema |
| `generate_document` | Generate documentation from the data model |

### Connecting External AI Tools

Any MCP-compatible client can connect to a ProcessGit MCP server:

```
MCP Server URL: https://your-processgit-instance.org/{owner}/{repo}/mcp
```

The server supports both standard HTTP request/response and SSE streaming for real-time tool execution results.

---

## AI Chat Agents

ProcessGit provides an in-browser AI chat interface tied to repository data. When a repository contains an `agent.chat.yaml` configuration, the file tree displays a clickable chat agent entry with a robot icon. Clicking it opens an interactive chat panel where users can ask questions about the repository data using natural language.

### Quick Start

Create an `agent.chat.yaml` in your repository root:

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

### Supported LLM Providers

| Provider | Example Models | API Key Env Var |
|----------|---------------|-----------------|
| **Anthropic** | `claude-sonnet-4-5`, `claude-haiku-3` | `ANTHROPIC_API_KEY` |
| **OpenAI** | `gpt-4o`, `gpt-4o-mini` | `OPENAI_API_KEY` |
| **Ollama** | `llama3`, `mistral` (local) | — (runs locally) |

### Agent File Discovery

| Priority | Path | Description |
|----------|------|-------------|
| 1 | `agent.chat.yaml` | Root directory (default agent) |
| 2 | `.processgit/agent.chat.yaml` | Config directory |
| 3 | `*.agent.chat.yaml` | Named variants (e.g., `classification.agent.chat.yaml`) |

Multiple `*.agent.chat.yaml` files create multiple independent chat agents in the same repository.

### MCP Tool Integration

Chat agents can use MCP tools to answer questions with data from the repository:

```yaml
mcp:
  use_repo_mcp: true            # Use this repo's own MCP server
  additional_servers:            # Connect to other repos' MCP servers
    - name: "org-register"
      url: "https://your-instance.org/owner/data-repo/mcp"
      description: "External data source"
  allowed_tools:                 # Whitelist specific tools
    - search
    - get_entity
    - describe_model
  denied_tools: []               # Or blacklist specific tools
```

### Conversation History

Enable persistent conversation storage on a dedicated git branch:

```yaml
history:
  enabled: true
  storage: "git-branch"
  branch: "chat-history"
  retention_days: 90
  max_conversations_per_user: 100
  anonymize: false
```

Conversations are stored in a date-organized structure on an orphan git branch, providing an immutable audit trail through git commit history. Commits are batched (every 5 minutes or 10+ updates) to avoid polluting history.

### Access Control & Rate Limiting

```yaml
access:
  visibility: "authenticated"     # "public", "authenticated", or "team"
  rate_limits:
    requests_per_minute: 10
    requests_per_day: 100
    max_conversation_turns: 50
  budget:
    max_monthly_usd: 50.00
    alert_threshold_pct: 80       # Alert admin at 80% budget usage
```

### Chat API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/{owner}/{repo}/chat` | Send a message (SSE stream response) |
| `GET` | `/{owner}/{repo}/chat/agents` | List available chat agents |
| `GET` | `/{owner}/{repo}/chat/history` | List conversation history |

### Server Configuration

Enable and configure chat agents globally in `app.ini`:

```ini
[chat]
ENABLED = true
MAX_AGENTS_PER_REPO = 10
RATE_LIMIT_PER_MINUTE = 10
MAX_MONTHLY_BUDGET = 100.0
DEFAULT_PROVIDER = anthropic
```

### Security Rules

- **API keys** are referenced by environment variable name only — never store actual keys in `agent.chat.yaml`
- **Rate limiting** is enforced per-user at both per-minute and per-day levels
- **Budget controls** stop serving requests when the monthly USD limit is exceeded
- **Visibility** controls who can access the chat (`public`, `authenticated`, or `team`)
- **Tool allow/deny lists** restrict which MCP tools the LLM can invoke
- **Iframe sandbox** applies to any custom viewer content rendered alongside chat

---

## Architecture Overview

- **Upstream:** Fork of [Gitea](https://github.com/go-gitea/gitea) (MIT), a self-hosted Git service written in Go
- **Backend:** Go — custom modules in `modules/diagrams`, `modules/uapf`, `modules/processgitviewer`
- **Frontend:** TypeScript + Webpack — diagram adapters in `web_src/js/features/diagrams/`, viewer framework in `web_src/js/features/processgitviewer/`
- **Diagram libraries:** [bpmn-js](https://bpmn.io/) (BPMN), cmmn-js (CMMN), dmn-js (DMN), bpmn-auto-layout
- **Deployment:** Docker-based with a bootstrap service for template repos
- **Database:** SQLite by default (with the `repo_classification` table for process metadata)
- **Validation:** Embedded JSON Schema for UAPF manifests; client-side XML validation for diagrams

---

## Installation

### Prerequisites

- Linux (Ubuntu 20.04+ recommended) or WSL2
- Docker Engine + Docker Compose (plugin)
- Git

### Quick Start

```bash
# Clone the repository
git clone https://github.com/Algomation-AI/ProcessGit.git
cd ProcessGit

# Build and start
docker compose -f deploy/docker-compose.yml up -d --build
```

The application will be available at `http://localhost:3000`.

### Verify Template Bootstrap

After starting, confirm the template repositories were created:

```bash
# Check bootstrap logs
docker compose -f deploy/docker-compose.yml logs -n 200 processgit-bootstrap

# Verify templates via API
curl -s "http://localhost:3000/api/v1/repo/search?q=&template=true" | head
```

The template dropdown in the New Repository UI should list starter templates for BPMN, CMMN, DMN, and UAPF packages.

### Redeploy

```bash
cd /home/ProcessGit
docker compose -f deploy/docker-compose.yml down
docker compose -f deploy/docker-compose.yml up -d --build
```

To force a bootstrap re-run:

```bash
docker compose -f deploy/docker-compose.yml exec processgit sh -lc \
  'rm -f /data/.processgit/templates_bootstrapped /data/.processgit/templates_token || true'
docker compose -f deploy/docker-compose.yml restart processgit-bootstrap
```

### Install Docker (Ubuntu / WSL)

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg

sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
  sudo tee /etc/apt/keyrings/docker.asc > /dev/null
sudo chmod a+r /etc/apt/keyrings/docker.asc

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
  https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
```

---

## Project Status

**Development / Early Access**

ProcessGit is under active development. Expect rapid iteration, breaking changes, and evolving documentation. Backward compatibility is not guaranteed at this stage.

---

## Contributing

Contributions are welcome. Please open an issue to discuss significant changes before submitting a pull request.

---

## License

ProcessGit is dual-licensed.

### Open Use (MIT)
Free for personal, academic, internal, and non-commercial use.

### Commercial Use
A commercial license is required for organizational, SaaS,
redistribution, or rebranded use.

Contact licensing@algomation.ai
