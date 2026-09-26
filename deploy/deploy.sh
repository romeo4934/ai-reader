#!/usr/bin/env bash
# Deploy ai-reader: build, install the binary, restart the service, gate on /ready.
#
#   cd /opt/ai-reader && git pull --ff-only && ./deploy/deploy.sh
set -euo pipefail

SERVICE=ai-reader
ROOT=/opt/$SERVICE
BIN=/usr/local/bin/$SERVICE

log() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31mERREUR:\033[0m %s\n' "$*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "à lancer en root"
[[ -f $ROOT/.env ]] || die "$ROOT/.env absent"

log "compilation"
cd "$ROOT"
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$BIN.new" ./cmd/$SERVICE
chmod 755 "$BIN.new"
mv -f "$BIN.new" "$BIN"

log "redémarrage du service"
systemctl restart "$SERVICE"

set -a
source "$ROOT/.env"
set +a
ADDR=${AI_READER_ADDR:-127.0.0.1:8091}

log "attente de /ready sur $ADDR"
READY=0
for _ in $(seq 1 40); do
  if curl -fsS --max-time 2 "http://$ADDR/ready" >/dev/null 2>&1; then
    READY=1; break
  fi
  sleep 0.5
done
if [[ $READY -ne 1 ]]; then
  journalctl -u "$SERVICE" -n 40 --no-pager || true
  die "le service ne répond pas sur /ready"
fi

log "déployé ✅"
curl -fsS --max-time 3 "http://$ADDR/ready" && echo
