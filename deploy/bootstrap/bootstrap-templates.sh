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

log() {
  printf '[templates-bootstrap] %s\n' "$1"
}

fatal() {
  log "ERROR: $1"
  exit 1
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fatal "Missing required command: $1"
  fi
}

if [ -f "$MARKER" ]; then
  log "Templates already bootstrapped; exiting."
  exit 0
fi

need_cmd curl
need_cmd git
need_cmd jq

[ -d "$TEMPLATE_ROOT" ] || fatal "Template root not found at $TEMPLATE_ROOT"
[ -f "$REPO_CONFIG" ] || fatal "Template config not found at $REPO_CONFIG"

mkdir -p "$(dirname "$MARKER")"

wait_for_config() {
  for _ in $(seq 1 60); do
    if [ -f "$CONFIG_PATH" ]; then
      return 0
    fi
    sleep 2
  done
  return 1
}

wait_for_http() {
  for _ in $(seq 1 60); do
    if curl -fsS "$API_BASE/version" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  return 1
}

log "Waiting for app config at $CONFIG_PATH"
wait_for_config || fatal "Gitea config not ready"

log "Waiting for ProcessGit API at $API_BASE"
wait_for_http || fatal "ProcessGit API did not respond"

export GITEA_WORK_DIR="$WORK_DIR"
export GITEA_CUSTOM="${GITEA_CUSTOM:-/data/gitea}"

gitea_cmd() {
  "$GITEA_BIN" --config "$CONFIG_PATH" "$@"
}

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

if [ -f "$TOKEN_FILE" ]; then
  TOKEN=$(cat "$TOKEN_FILE")
  log "Reusing existing token from $TOKEN_FILE"
else
  token_name="processgit-templates-bootstrap-$(date +%s)"
  log "Generating access token ($token_name)"
  TOKEN=$(gitea_cmd admin user generate-access-token --username "$OWNER" --token-name "$token_name" --scopes "all" --raw 2>/dev/null || true)
  if [ -z "$TOKEN" ]; then
    fatal "Failed to generate access token for $OWNER"
  fi
  printf '%s' "$TOKEN" > "$TOKEN_FILE"
  chmod 600 "$TOKEN_FILE"
fi

create_repo_if_missing() {
  name="$1"
  title="$2"
  path="$3"
  description="$4"

  repo_api="$API_BASE/repos/$OWNER/$name"
  if curl -fsS -H "Authorization: token $TOKEN" "$repo_api" >/dev/null 2>&1; then
    log "Repo $OWNER/$name already exists"
    exists=1
    curl -fsS -X PATCH -H "Authorization: token $TOKEN" \
      -H 'Content-Type: application/json' \
      -d "{\"description\":\"$description\",\"template\":true,\"private\":false}" \
      "$repo_api" >/dev/null || log "Unable to update metadata for $name (continuing)"
  else
    exists=0
  fi

  if [ "$exists" -eq 0 ]; then
    log "Creating repo $OWNER/$name"
    curl -fsS -H "Authorization: token $TOKEN" \
      -H 'Content-Type: application/json' \
      -X POST \
      -d "{\"name\":\"$name\",\"description\":\"$description\",\"private\":false,\"template\":true,\"auto_init\":false,\"default_branch\":\"main\"}" \
      "$API_BASE/user/repos" >/dev/null || fatal "Failed to create repository $name"
  fi

  remote="http://$OWNER:$TOKEN@processgit:3000/$OWNER/$name.git"
  if git ls-remote "$remote" >/dev/null 2>&1; then
    if git ls-remote "$remote" | grep -q .; then
      log "Repo $name already has content; skipping push"
      return
    fi
  fi

  src="$TEMPLATE_ROOT/$path"
  [ -d "$src" ] || fatal "Template source missing: $src"

  tmp_dir=$(mktemp -d)
  trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM
  cp -a "$src/." "$tmp_dir/"
  (cd "$tmp_dir" && { git init -b main 2>/dev/null || { git init && git checkout -B main; }; } && git config user.name "$OWNER" && git config user.email "$OWNER_EMAIL" && git add . && git commit -m "Initial template import" && GIT_TERMINAL_PROMPT=0 git push "$remote" main:main)
  rm -rf "$tmp_dir"
  trap - EXIT HUP INT TERM
}

log "Bootstrapping template repositories"
while IFS= read -r entry; do
  name=$(printf '%s' "$entry" | jq -r '.name')
  title=$(printf '%s' "$entry" | jq -r '.title')
  path=$(printf '%s' "$entry" | jq -r '.path')
  description=$(printf '%s' "$entry" | jq -r '.description')
  create_repo_if_missing "$name" "$title" "$path" "$description"
done <<EOF_ENTRIES
$(jq -c '.[]' "$REPO_CONFIG")
EOF_ENTRIES

touch "$MARKER"
log "Template bootstrap completed"
