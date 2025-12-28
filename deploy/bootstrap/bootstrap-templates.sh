#!/usr/bin/env sh
set -eu

MARKER="/data/.processgit/templates_bootstrapped"
TEMPLATE_ROOT="/opt/processgit/repo-templates"
REPO_CONFIG="/opt/processgit/bootstrap/template-repos.json"

OWNER="${PROCESSGIT_TEMPLATES_OWNER:-processgit-templates}"
OWNER_EMAIL="${PROCESSGIT_TEMPLATES_EMAIL:-processgit-templates@example.invalid}"
OWNER_PASSWORD="${PROCESSGIT_TEMPLATES_PASSWORD:-processgit-templates}"

API_BASE="${PROCESSGIT_API_BASE:-http://processgit:3000/api/v1}"
ADMIN_TOKEN="${PROCESSGIT_ADMIN_TOKEN:-}"

log() { printf '[templates-bootstrap] %s\n' "$1"; }
fatal() { log "ERROR: $1"; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || fatal "Missing command: $1"; }
need curl
need jq
need git

[ -d "$TEMPLATE_ROOT" ] || fatal "Template root not found at $TEMPLATE_ROOT"
[ -f "$REPO_CONFIG" ] || fatal "Template repo config not found at $REPO_CONFIG"
jq -e type "$REPO_CONFIG" >/dev/null 2>&1 || fatal "Invalid JSON in $REPO_CONFIG"

if [ -f "$MARKER" ]; then
  log "Templates already bootstrapped; exiting."
  exit 0
fi

[ -n "$ADMIN_TOKEN" ] || fatal "PROCESSGIT_ADMIN_TOKEN is not set"

wait_for_http() {
  url="$1"
  i=0
  while [ $i -lt 120 ]; do
    if curl -fsS "$url" >/dev/null 2>&1; then return 0; fi
    i=$((i+1))
    sleep 1
  done
  return 1
}

log "Waiting for API at $API_BASE/version"
wait_for_http "$API_BASE/version" || fatal "API did not respond"

api() {
  method="$1"; url="$2"; data="${3:-}"
  if [ -n "$data" ]; then
    curl -fsS -X "$method" \
      -H "Authorization: token $ADMIN_TOKEN" \
      -H "Content-Type: application/json" \
      -d "$data" "$url"
  else
    curl -fsS -X "$method" \
      -H "Authorization: token $ADMIN_TOKEN" "$url"
  fi
}

ensure_user() {
  if api GET "$API_BASE/users/$OWNER" >/dev/null 2>&1; then
    log "Templates user '$OWNER' already exists"
    return 0
  fi

  log "Creating templates user '$OWNER' via admin API"
  payload="$(jq -nc \
    --arg u "$OWNER" --arg e "$OWNER_EMAIL" --arg p "$OWNER_PASSWORD" \
    '{username:$u,email:$e,password:$p,must_change_password:false,send_notify:false,restricted:false,visibility:"public"}')"

  api POST "$API_BASE/admin/users" "$payload" >/dev/null || fatal "Failed to create templates user via API"
}

# Create token for templates user (admin endpoint)
ensure_user_token() {
  # Try to list tokens: if not supported, we create new each run and store in /data
  TOK_FILE="/data/.processgit/templates_user_token"
  if [ -f "$TOK_FILE" ]; then
    cat "$TOK_FILE"
    return 0
  fi

  log "Creating access token for '$OWNER' via admin API"
  payload="$(jq -nc --arg n "templates-bootstrap" '{name:$n, scopes:["all"]}')"

  # Admin endpoint to create token for user:
  # POST /admin/users/{username}/tokens
  # Response contains "sha1" token in many Gitea versions.
  resp="$(api POST "$API_BASE/admin/users/$OWNER/tokens" "$payload" || true)"
  token="$(printf '%s' "$resp" | jq -r '.sha1 // .token // empty')"

  [ -n "$token" ] || fatal "Could not create templates token; API response: $resp"

  mkdir -p /data/.processgit
  printf '%s' "$token" > "$TOK_FILE"
  chmod 600 "$TOK_FILE"
  printf '%s' "$token"
}

ensure_repo() {
  name="$1"; description="$2"
  if api GET "$API_BASE/repos/$OWNER/$name" >/dev/null 2>&1; then
    log "Repo $OWNER/$name exists; ensuring template flag"
    patch="$(jq -nc --arg d "$description" '{description:$d, template:true, private:false}')"
    api PATCH "$API_BASE/repos/$OWNER/$name" "$patch" >/dev/null || fatal "Failed to patch repo $OWNER/$name"
  else
    log "Creating repo $OWNER/$name (template=true)"
    payload="$(jq -nc --arg n "$name" --arg d "$description" \
      '{name:$n, description:$d, private:false, template:true, auto_init:false, default_branch:"main"}')"
    # Create repo as the templates user using admin token endpoint:
    api POST "$API_BASE/admin/users/$OWNER/repos" "$payload" >/dev/null || fatal "Failed to create repo $OWNER/$name"
  fi
}

push_content_if_empty() {
  name="$1"; src="$2"; user_token="$3"
  remote="http://$OWNER:$user_token@processgit:3000/$OWNER/$name.git"

  # If main branch exists, skip
  if git ls-remote "$remote" 2>/dev/null | grep -q 'refs/heads/'; then
    log "Repo $OWNER/$name already has content; skipping push"
    return 0
  fi

  log "Pushing template content into $OWNER/$name"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT HUP INT TERM
  cp -a "$src/." "$tmp/"

  ( cd "$tmp"
    git init -b main
    git config user.email "templates@processgit.org"
    git config user.name "ProcessGit Templates"
    git add -A
    git commit -m "Initial template import"
    GIT_TERMINAL_PROMPT=0 git push "$remote" main:main
  ) || fatal "Failed to push content for $OWNER/$name"

  rm -rf "$tmp"
  trap - EXIT HUP INT TERM
}

main() {
  ensure_user
  user_token="$(ensure_user_token)"

  log "Bootstrapping template repos from $REPO_CONFIG"
  jq -c '.[]' "$REPO_CONFIG" | while IFS= read -r entry; do
    name="$(printf '%s' "$entry" | jq -r '.name')"
    path="$(printf '%s' "$entry" | jq -r '.path')"
    desc="$(printf '%s' "$entry" | jq -r '.description')"

    src="$TEMPLATE_ROOT/$path"
    [ -d "$src" ] || fatal "Template content folder missing: $src"

    ensure_repo "$name" "$desc"
    push_content_if_empty "$name" "$src" "$user_token"
  done

  mkdir -p /data/.processgit
  touch "$MARKER"
  log "Template bootstrap completed"
}

main
