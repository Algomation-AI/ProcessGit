# ProcessGit

[![GitHub tag](https://img.shields.io/github/v/tag/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/tags)
[![GitHub license](https://img.shields.io/github/license/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/blob/main/LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/issues)
[![GitHub stars](https://img.shields.io/github/stars/Algomation-AI/ProcessGit)](https://github.com/Algomation-AI/ProcessGit/stargazers)

**ProcessGit** is a Git-based repository system for **executable processes and algorithms**.

It brings long-established software-engineering discipline — version control, review, releases, traceability — to **process logic**, workflows, and algorithmic definitions that traditionally live in documents, diagrams, or proprietary tools.

 **Public demo / test instance:**  
 https://processgit.org

> **Important notice**  
> The public instance is **fully functional**, but intended for **testing, evaluation, and demonstration only**.

---

## What is ProcessGit

ProcessGit is a **Process Repository** (sometimes referred to as a *processpository*).

It is designed to store, manage, and publish **machine-executable process artifacts** using the same principles that have proven themselves in source-code management for decades.

ProcessGit allows you to:
- Store executable processes as versioned artifacts
- Review and evolve processes through commits and history
- Tag and release stable process versions
- Share and distribute process logic in a controlled way
- Treat organizational workflows as **first-class assets**

Instead of managing only *source code*, ProcessGit manages **how work is done**.

---

## Typical Use Cases

- Executable business processes (e.g. BPMN-based workflows)
- Decision logic and rulesets (e.g. DMN-style models)
- Algorithm packages (e.g. `.uapf` files)
- Public-sector and enterprise process catalogs
- AI-assisted execution engines that require governed inputs
- Single Source of Truth (SSOT) for operational logic

---

## Public Demo Instance

The public instance at **https://processgit.org** is provided to:
- Explore the UI and core concepts
- Test repository creation and browsing
- Demonstrate the process-as-code approach

### Development Disclaimer

- This instance runs **active development builds**
- Features, APIs, and data **may change without notice**
- Data may be reset or removed at any time
- No availability or data durability guarantees are provided

**Use at your own risk. No warranties. No service guarantees.**

---

## Project Status

**Development / Early Access**

ProcessGit is under active development.  
Expect:
- Rapid iteration
- Breaking changes
- Incomplete or evolving documentation

Backward compatibility is **not guaranteed** at this stage.

---

## Architecture Overview (High Level)

- Web application (UI + API)
- Git-like repository abstraction for processes
- Docker-based deployment
- Designed to integrate with external execution engines
- Compatible with algorithm packaging standards (e.g. UAPF)

## Repository Classification (Platform Metadata)

ProcessGit tracks repository classification directly in the platform database (SQLite by default) rather than in Git content. Each repository has metadata fields for `repo_type` (process, decision, reference, connector), `uapf_level` (0–4 or n/a), optional `reference_kind` (when the type is reference), and `status` (draft/stable/deprecated/archived). A default classification entry is created when a repository is created (currently `repo_type=process`, `status=draft`). Future releases will add UI badges and editing, enabling clearer distinction between process vs. reference repos, dependency governance, and UAPF-specific flows.

---

## Installation (From Scratch)

### Prerequisites

- Linux (Ubuntu 20.04+ recommended) or WSL2
- Docker Engine
- Docker Compose (plugin)
- Git

---

## Template bootstrap verification (self-hosted)

After starting the stack, confirm the templates are created, public, and marked as templates:

```bash
# Build and start (from repo root)
docker compose -f deploy/docker-compose.yml up -d --build

# Check bootstrap logs
docker compose -f deploy/docker-compose.yml logs -n 200 processgit-bootstrap

# Verify template repositories via API
curl -s "http://localhost:3000/api/v1/repo/search?q=&template=true" | head

# Inspect template user repos/flags from inside the container
docker exec -it processgit sh -lc '
TOKEN=$(cat /data/.processgit/templates_token 2>/dev/null || true)
[ -n "$TOKEN" ] || exit 1
curl -s -H "Authorization: token $TOKEN" http://localhost:3000/api/v1/users/processgit-templates/repos | jq -r ".[] | [.name, .template, .private] | @tsv"
'
```

The template dropdown in the UI should list the repositories owned by `processgit-templates`, and each should show `template=true` and `private=false` in the API output.

### Redeploy and bootstrap reset

After updating the deployment manifests, rebuild and restart the stack:

```bash
cd /home/ProcessGit
docker compose -f deploy/docker-compose.yml down
docker compose -f deploy/docker-compose.yml up -d --build

docker compose -f deploy/docker-compose.yml logs -n 200 processgit
docker compose -f deploy/docker-compose.yml logs -n 200 processgit-bootstrap
```

To rerun bootstrap when markers already exist:

```bash
docker compose -f deploy/docker-compose.yml exec processgit sh -lc 'rm -f /data/.processgit/templates_bootstrapped /data/.processgit/templates_token || true'
docker compose -f deploy/docker-compose.yml restart processgit-bootstrap
docker compose -f deploy/docker-compose.yml logs -n 200 processgit-bootstrap
```

Verify templates via the API:

```bash
docker compose -f deploy/docker-compose.yml exec processgit sh -lc 'curl -s http://localhost:3000/api/v1/repo/search?q=&template=true | head -c 1000; echo'
```

### 1. Install Docker

#### Ubuntu / WSL (recommended)
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

