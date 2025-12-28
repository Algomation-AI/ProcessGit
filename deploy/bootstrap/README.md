# ProcessGit bootstrap

The bootstrap job seeds template repositories during local development or initial deployments. It relies on a `deploy/.env` file that provides the admin token used to call the ProcessGit API.

## Required environment

1. Create `deploy/.env` next to `deploy/docker-compose.yml`.
2. Add the admin token as `PROCESSGIT_ADMIN_TOKEN=<your-token>`.
3. Keep this file local—**do not commit the token**.

## Rerunning the bootstrap

The bootstrap script writes a marker at `/data/.processgit/templates_bootstrapped`. To force a fresh run:

```bash
rm -f /data/.processgit/templates_bootstrapped
docker compose -f deploy/docker-compose.yml up processgit-bootstrap
```
