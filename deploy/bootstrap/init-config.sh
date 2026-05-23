#!/usr/bin/env sh
# Bootstrap Gitea's app.ini before the main container starts.
#
# Why: Gitea, when no app.ini is present, falls through to its interactive
# install wizard. The container then never satisfies the
# /api/v1/version healthcheck (which is not served in install mode), the
# compose orchestration declares it unhealthy, and the
# processgit-bootstrap step that waits on healthy is deadlocked.
#
# This script is the deterministic alternative to depending on the
# upstream gitea/gitea image's s6 startup conventions (which moved
# between s6-overlay v2 and v3 and broke our path-based override of
# /etc/s6/gitea/run, silently skipping the env-var-to-ini conversion
# the image would otherwise do).
#
# Idempotent: if app.ini already exists with INSTALL_LOCK = true, do
# nothing. So restarts and updates don't clobber the operator's tuned
# config or the secrets generated on first boot.

set -eu

CONF=/data/gitea/conf/app.ini

if [ -f "$CONF" ] && grep -q '^INSTALL_LOCK *= *true' "$CONF"; then
  echo "[init-config] $CONF exists and is locked; skipping"
  exit 0
fi

echo "[init-config] generating $CONF"

mkdir -p \
  /data/gitea/conf \
  /data/gitea/log \
  /data/gitea/attachments \
  /data/gitea/avatars \
  /data/gitea/repo-avatars \
  /data/gitea/sessions \
  /data/gitea/indexers \
  /data/git/repositories \
  /data/git/lfs \
  /data/git/.ssh

# Gitea's RewriteAllPublicKeys() writes authorized_keys.tmp here on
# every startup (even with zero registered keys, it writes an empty
# file). SSH requires the parent dir to be 0700 owned by the running
# user; otherwise it fails closed with "permission denied".
chmod 0700 /data/git/.ssh

# Generate per-deployment secrets using the bundled gitea binary. These
# are written into the file once and never regenerated — losing them
# would invalidate all existing sessions, signed cookies, and lfs JWT
# tokens, so the idempotent guard above protects them.
SECRET_KEY="$(/app/gitea/gitea generate secret SECRET_KEY)"
INTERNAL_TOKEN="$(/app/gitea/gitea generate secret INTERNAL_TOKEN)"
JWT_SECRET="$(/app/gitea/gitea generate secret JWT_SECRET)"
LFS_JWT_SECRET="$(/app/gitea/gitea generate secret LFS_JWT_SECRET)"

# Operator-overridable values; defaults are sane for `docker compose up`
# on localhost with the published port mapping (18080:3000, 12222:22).
APP_NAME="${APP_NAME:-ProcessGit}"
DOMAIN="${PROCESSGIT_DOMAIN:-localhost}"
ROOT_URL="${PROCESSGIT_ROOT_URL:-http://localhost:18080/}"
SSH_PORT="${PROCESSGIT_SSH_PORT:-12222}"

cat > "$CONF" <<EOF
APP_NAME = ${APP_NAME}
RUN_USER = git
RUN_MODE = prod

[server]
PROTOCOL = http
DOMAIN = ${DOMAIN}
HTTP_PORT = 3000
ROOT_URL = ${ROOT_URL}
SSH_DOMAIN = ${DOMAIN}
SSH_PORT = ${SSH_PORT}
SSH_LISTEN_PORT = 22
LFS_START_SERVER = true
OFFLINE_MODE = true
DISABLE_ROUTER_LOG = false

[database]
DB_TYPE = sqlite3
PATH = /data/gitea/gitea.db
LOG_SQL = false

[indexer]
ISSUE_INDEXER_PATH = /data/gitea/indexers/issues.bleve

[session]
PROVIDER = file
PROVIDER_CONFIG = /data/gitea/sessions

[picture]
AVATAR_UPLOAD_PATH = /data/gitea/avatars
REPOSITORY_AVATAR_UPLOAD_PATH = /data/gitea/repo-avatars
DISABLE_GRAVATAR = false
ENABLE_FEDERATED_AVATAR = false

[attachment]
PATH = /data/gitea/attachments

[log]
ROOT_PATH = /data/gitea/log
MODE = console
LEVEL = info

[security]
INSTALL_LOCK = true
SECRET_KEY = ${SECRET_KEY}
INTERNAL_TOKEN = ${INTERNAL_TOKEN}
PASSWORD_HASH_ALGO = pbkdf2_v2

[oauth2]
JWT_SECRET = ${JWT_SECRET}

[lfs]
PATH = /data/git/lfs
JWT_SECRET = ${LFS_JWT_SECRET}

[service]
DISABLE_REGISTRATION = false
REQUIRE_SIGNIN_VIEW = false
REGISTER_EMAIL_CONFIRM = false
ENABLE_NOTIFY_MAIL = false
ALLOW_ONLY_EXTERNAL_REGISTRATION = false
ENABLE_CAPTCHA = false
DEFAULT_KEEP_EMAIL_PRIVATE = false
DEFAULT_ALLOW_CREATE_ORGANIZATION = true
DEFAULT_ENABLE_TIMETRACKING = true
NO_REPLY_ADDRESS = noreply.localhost

[mailer]
ENABLED = false

[openid]
ENABLE_OPENID_SIGNIN = false
ENABLE_OPENID_SIGNUP = false

[cron.update_checker]
ENABLED = false

[repository]
ROOT = /data/git/repositories

[mirror]
DEFAULT_INTERVAL = 8h
EOF

chmod 0640 "$CONF"

echo "[init-config] wrote $CONF ($(wc -c < "$CONF") bytes)"
echo "[init-config] APP_NAME=${APP_NAME} DOMAIN=${DOMAIN} ROOT_URL=${ROOT_URL} SSH_PORT=${SSH_PORT}"
