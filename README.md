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
- Import and export UAPF algorithm packages with schema validation
- Ship custom HTML viewers/editors alongside any data file using a simple manifest
- Classify repositories by type (process, decision, reference, connector) and lifecycle status
- Tag, release, and review process definitions through commits and pull requests
- Treat organizational workflows as **first-class governed assets**

Instead of managing only source code, ProcessGit manages **how work is done**.

---

## Core Features

### 1. BPMN / CMMN / DMN Diagram Viewer & Editor

ProcessGit natively detects and renders OMG process modeling files. When you navigate to a supported file, the platform replaces the raw source view with an interactive graphical canvas.

**Supported formats:**

| Standard | File extensions | Capabilities |
|----------|----------------|-------------|
| BPMN 2.0 | `.bpmn`, `.bpmn20.xml`, `*bpmn.xml` | View + Edit + Properties panel |
| CMMN 1.1 | `.cmmn`, `.cmmn11.xml`, `*cmmn.xml` | View + Edit + Properties panel |
| DMN 1.3 | `.dmn`, `.dmn11.xml`, `*dmn.xml` | View + Edit + Properties panel |
| N-Graph | `.ngraph.json`, `.ngraph.xml`, `.ngraph` | View |
| Ruleset | `.ruleset.json`, `.ruleset.dmn`, `.ruleset` | View |

**Detection** works by file extension first and falls back to content sniffing (e.g. checking for `xmlns:bpmn=` in the XML). BPMN files missing diagram layout information (`bpmndi:BPMNDiagram`) are auto-laid-out before rendering.

**Three-mode toolbar** — every diagram file gets a toolbar with *Preview*, *Edit*, and *Raw* tabs. Preview renders the graphical diagram (read-only, fitted to viewport). Edit opens a full modeler with a properties panel for BPMN/CMMN/DMN (powered by [bpmn-js](https://bpmn.io/), cmmn-js, and dmn-js). Raw shows the underlying XML/JSON source. A **Save** button commits changes directly to the repository branch.

**Template repositories** — ProcessGit bootstraps a set of starter templates (single BPMN process, single DMN decision, single CMMN case, and several UAPF variants) that appear in the "New Repository" template dropdown.

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

#### Real-World Example: VDVC Organization Register Viewer

The [Organizations Register](https://processgit.org) repository demonstrates a custom viewer for an XML register of Latvian government organizations, validated against an XSD schema.

**Repository structure:**
```
processgit.viewer.json          ← Manifest
vdvc-register-viewer.html       ← Custom HTML viewer/editor
vdvc-register.xml               ← Data file (the register)
vdvc-register.xsd               ← XSD schema for validation
```

**Manifest:**
```json
{
  "version": 1,
  "viewers": [
    {
      "id": "vdvc-register",
      "primary_pattern": "vdvc-register.xml",
      "type": "html",
      "entry": "vdvc-register-viewer.html",
      "edit_allow": ["vdvc-register.xml"],
      "targets": {
        "xsd": "vdvc-register.xsd",
        "xml": "vdvc-register.xml"
      }
    }
  ]
}
```

When a user browses to `vdvc-register.xml`, instead of seeing raw XML, they get a fully interactive table editor with search, sorting, inline editing, XSD validation status, and the ability to save changes back to the repository — all from a single self-contained HTML file shipped in the same repo.

---

### 4. XSD Schema Visualizer

ProcessGit includes a built-in visual viewer for XML Schema Definition (`.xsd`) files. Instead of reading raw XSD XML, users see an interactive graph-based representation of the schema structure showing elements, complex types, simple types, relationships, and constraints.

The XSD visualizer supports:

- Parsing and rendering schema element hierarchies
- Visual graph layout of type relationships
- Interactive editing (adding child elements, renaming elements/types, setting occurrence constraints, editing documentation annotations)
- Serializing changes back to valid XSD XML
- Export capabilities

---

### 5. DVS XML Viewer

ProcessGit includes a dedicated viewer for DVS (Document and Value Set) XML classification schemes and document metadata. This provides structured preview, edit, and raw views for classification and document metadata XML files used in government and enterprise contexts.

---

### 6. Repository Classification

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
- XSD schemas with visual browsing
- AI-assisted execution engines that require governed, versioned inputs
- Single Source of Truth (SSOT) for operational logic

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
