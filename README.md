# ProcessGit

**ProcessGit** is a Git for Digital Process storage for **BPMN / DMN / CMMN** and **UAPF** packages — designed to bring “GitHub-like” collaboration to operational process assets:
- diagram-friendly previews
- process-aware diffs in pull requests
- validation gates for UAPF + models
- versioned releases of process packages

> POC status: this repository currently starts as a fork base for a ProcessGit distribution built on top of Gitea.

## Why ProcessGit
Traditional Git for XML diagrams works, but it’s missing the things process teams actually need:
- semantic diffs (not just XML diffs)
- consistent packaging and release of executable process bundles (UAPF)
- process-centric metadata (owners, systems, connectors, runtime targets)

ProcessGit keeps the proven Git workflow (branches, PRs, tags) and adds process-native UX.

## Upstream
ProcessGit is based on **Gitea** (MIT license).

- Upstream: Gitea
- Source: https://github.com/go-gitea/gitea
- License: see [LICENSE](./LICENSE)
- Attribution: see [NOTICE](./NOTICE)

## POC goals (near-term)
1. Rebranded UI/CLI distribution (“ProcessGit”)
2. Repository file preview for `.bpmn`, `.dmn`, `.cmmn`, `.uapf`
3. Pull request checks for process validation (UAPF structure + lint rules)
4. Diagram-aware diffs for BPMN changes

## Quick start (POC)
A minimal Docker compose can be found under `deploy/` (initially uses the stock Gitea image for infrastructure validation).

```bash
docker compose -f deploy/docker-compose.yml up -d
```

Then open:

http://localhost:3000

## License

This repository contains work derived from Gitea and is distributed under the MIT License.
See LICENSE and NOTICE.
