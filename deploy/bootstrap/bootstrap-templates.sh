#!/usr/bin/env sh
set -eu

MARKER="/data/.processgit/templates_bootstrapped"
TOKEN_FILE="/data/.processgit/templates_token"
TEMPLATE_ROOT="/opt/processgit/repo-templates"
REPO_CONFIG="/opt/processgit/bootstrap/template-repos.json"

OWNER="${PROCESSGIT_TEMPLATES_OWNER:-processgit-templates}"
OWNER_EMAIL="${PROCESSGIT_TEMPLATES_EMAIL:-processgit-templates@example.invalid}"
OWNER_PASSWORD="${PROCESSGIT_TEMPLATES_PASSWORD:-processgit-templates}"

GITEA_BIN="${GITEA_BIN:-/app/gitea/gitea}"
CONFIG_PATH="${GITEA_CUSTOM:-/data/gitea}/conf/app.ini"
API_BASE="${PROCESSGIT_API_BASE:-http://processgit:3000/api/v1}"
WORK_DIR="${GITEA_WORK_DIR:-/data}"

log() { printf '[templates-bootstrap] %s\n' "$1"; }
fatal() { log "ERROR: $1"; exit 1; }
need_cmd() { command -v "$1" >/dev/null 2>&1 || fatal "Missing required command: $1"; }

if [ -f "$MARKER" ]; then
  log "Templates already bootstrapped; exiting."
  exit 0
fi

need_cmd curl
need_cmd git
need_cmd jq

[ -d "$TEMPLATE_ROOT" ] || fatal "Template root not found at $TEMPLATE_ROOT"
[ -f "$REPO_CONFIG" ] || fatal "Template repo config not found at $REPO_CONFIG"
jq -e type "$REPO_CONFIG" >/dev/null 2>&1 || fatal "Invalid JSON in $REPO_CONFIG"

export GITEA_WORK_DIR="$WORK_DIR"
export GITEA_CUSTOM="${GITEA_CUSTOM:-/data/gitea}"

gitea_cmd() {
  "$GITEA_BIN" --config "$CONFIG_PATH" "$@"
}

wait_for_file() {
  p="$1"
  i=0
  while [ $i -lt 120 ]; do
    [ -f "$p" ] && return 0
    i=$((i+1))
    sleep 1
  done
  return 1
}

wait_for_http() {
  url="$1"
  i=0
  while [ $i -lt 120 ]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    i=$((i+1))
    sleep 1
  done
  return 1
}

log "Waiting for app config at $CONFIG_PATH"
wait_for_file "$CONFIG_PATH" || fatal "Gitea config not ready"

log "Waiting for ProcessGit API at $API_BASE/version"
wait_for_http "$API_BASE/version" || fatal "ProcessGit API did not respond"

# Ensure templates user exists
user_exists() {
  gitea_cmd admin user list | awk -v user="$OWNER" 'NR>1 && $2==user {found=1} END{exit found?0:1}'
}

if user_exists; then
  log "Templates owner '$OWNER' already exists"
else
  log "Creating templates owner '$OWNER'"
  gitea_cmd admin user create \
    --username "$OWNER" \
    --password "$OWNER_PASSWORD" \
    --email "$OWNER_EMAIL" \
    --must-change-password=false || fatal "Failed to create templates user"
fi

# Obtain (or generate) a token for templates user
TOKEN=""
if [ -f "$TOKEN_FILE" ]; then
  TOKEN="$(cat "$TOKEN_FILE" || true)"
fi

if [ -z "$TOKEN" ]; then
  token_name="processgit-templates-bootstrap"
  log "Generating access token for '$OWNER' ($token_name)"

  # Correct CLI syntax per Gitea docs:
  # gitea admin user generate-access-token --username <user> --token-name <name> --scopes all --raw
  TOKEN="$(gitea_cmd admin user generate-access-token \
    --username "$OWNER" \
    --token-name "$token_name" \
    --scopes all \
    --raw 2>/dev/null || true)"

  [ -n "$TOKEN" ] || fatal "Failed to generate access token for $OWNER"

  mkdir -p "$(dirname "$TOKEN_FILE")"
  printf '%s' "$TOKEN" > "$TOKEN_FILE"
  chmod 600 "$TOKEN_FILE"
else
  log "Reusing existing token from $TOKEN_FILE"
fi

api() {
  method="$1"; url="$2"; data="${3:-}"
  if [ -n "$data" ]; then
    curl -fsS -X "$method" \
      -H "Authorization: token $TOKEN" \
      -H "Content-Type: application/json" \
      -d "$data" \
      "$url"
  else
    curl -fsS -X "$method" \
      -H "Authorization: token $TOKEN" \
      "$url"
  fi
}

create_or_update_repo() {
  name="$1"
  title="$2"
  path="$3"
  description="$4"

  src="$TEMPLATE_ROOT/$path"
  [ -d "$src" ] || fatal "Template content folder missing: $src"

  repo_api="$API_BASE/repos/$OWNER/$name"

  if api GET "$repo_api" >/dev/null 2>&1; then
    log "Repo $OWNER/$name exists; ensuring template flag"
    patch="$(jq -nc --arg desc "$description" '{description:$desc, template:true, private:false}')"
    api PATCH "$repo_api" "$patch" >/dev/null || fatal "Failed to PATCH repo $OWNER/$name"
  else
    log "Creating repo $OWNER/$name (template=true)"
    payload="$(jq -nc \
      --arg name "$name" \
      --arg desc "$description" \
      '{name:$name, description:$desc, private:false, template:true, auto_init:false, default_branch:"main"}')"
    api POST "$API_BASE/user/repos" "$payload" >/dev/null || fatal "Failed to create repository $OWNER/$name"
  fi

  remote="http://$OWNER:$TOKEN@processgit:3000/$OWNER/$name.git"

  # If already has commits, leave it alone
  if git ls-remote "$remote" >/dev/null 2>&1; then
    if git ls-remote "$remote" | grep -q 'refs/heads/'; then
      log "Repo $OWNER/$name already has content; skipping push"
      return 0
    fi
  fi

  log "Pushing template content to $OWNER/$name"
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

  cp -a "$src/." "$tmp_dir/"

  ( cd "$tmp_dir"
    git init -b main
    git config user.email "templates@processgit.org"
    git config user.name "ProcessGit Templates"
    git add -A
    git commit -m "Initial template import"
    GIT_TERMINAL_PROMPT=0 git push "$remote" main:main
  ) || fatal "Failed to push template content for $OWNER/$name"

  rm -rf "$tmp_dir"
  trap - EXIT HUP INT TERM
}

log "Bootstrapping template repositories from $REPO_CONFIG"
ok=1
jq -c '.[]' "$REPO_CONFIG" | while IFS= read -r entry; do
  name="$(printf '%s' "$entry" | jq -r '.name')"
  title="$(printf '%s' "$entry" | jq -r '.title')"
  path="$(printf '%s' "$entry" | jq -r '.path')"
  description="$(printf '%s' "$entry" | jq -r '.description')"
  create_or_update_repo "$name" "$title" "$path" "$description"
done

mkdir -p "$(dirname "$MARKER")"
touch "$MARKER"
log "Template bootstrap completed"
