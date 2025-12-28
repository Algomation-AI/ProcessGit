#!/usr/bin/env sh
set -eu

log() {
  printf '%s\n' "$1"
}

log "Checking required files"
REQUIRED_PATHS="enterprise/enterprise.index.json packages"
for path in $REQUIRED_PATHS; do
  if [ ! -e "$path" ]; then
    log "Missing required path: $path"
    exit 1
  fi
done

log "Validating XML assets when xmllint is available"
if command -v xmllint >/dev/null 2>&1; then
  find . -type f \( -name '*.bpmn' -o -name '*.dmn' -o -name '*.cmmn' \) -print0 |
    xargs -0 -r -n1 xmllint --noout
else
  log "xmllint not installed; skipping XML validation"
fi

log "Validation complete"
