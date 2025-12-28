# ProcessGit bootstrap

The bootstrap job seeds template repositories during local development or initial deployments. It reads the admin token from the repository root `.env` file (referenced from this directory as `../.env`) and uses it only to call the ProcessGit API for template creation.

## Required environment

1. Copy `.env.example` to `.env` at the repository root.
2. Create an admin token in the UI: **Settings → Applications → Access Token**.
3. Add the token to `../.env` as `PROCESSGIT_ADMIN_TOKEN=<your-token>` (used only by the bootstrap container to create template repos).
4. Keep `.env` local—**do not commit the token**.

## Rerunning the bootstrap

The bootstrap script writes marker files at `/data/.processgit/templates_bootstrapped` and `/data/.processgit/templates_user_token`. Remove them to force a fresh run after updating your token or templates.

## Operator runbook

```bash
# 1) Create root .env from example
cp .env.example .env
nano .env   # paste token

# 2) Deploy
docker compose -f deploy/docker-compose.yml up -d --build

# 3) If you need to rerun bootstrap
docker compose -f deploy/docker-compose.yml exec processgit sh -lc \
  'rm -f /data/.processgit/templates_bootstrapped /data/.processgit/templates_user_token || true'
docker compose -f deploy/docker-compose.yml restart processgit-bootstrap
docker compose -f deploy/docker-compose.yml logs -n 200 processgit-bootstrap
```
