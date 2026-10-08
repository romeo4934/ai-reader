#!/usr/bin/env bash
# Nightly backup: a consistent SQLite copy, then an off-box copy to Backblaze B2.
#
# The database is copied with SQLite's own backup mechanism, never with cp — a
# file copy of a live WAL database can be torn and silently unrestorable.
# Runs as the ai-reader user so SQLite never leaves root-owned -wal/-shm files
# next to the live database. B2 credentials come from the service .env; no
# rclone.conf is written.
set -euo pipefail

SERVICE=ai-reader
ROOT=/opt/$SERVICE
DB=${AI_READER_DB:-$ROOT/data/$SERVICE.sqlite}
DIR=$ROOT/backups
KEEP_DAYS=14

log() { printf '[%s] %s\n' "$(date -Is)" "$*"; }
die() { printf '[%s] ERREUR: %s\n' "$(date -Is)" "$*" >&2; exit 1; }

[[ -f $DB ]] || die "base absente: $DB"
[[ -d $DIR && -w $DIR ]] || die "dossier de sauvegarde absent ou non inscriptible: $DIR"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT=$DIR/$SERVICE-$STAMP.sqlite

log "sauvegarde vers $OUT"
sqlite3 "$DB" ".timeout 5000" ".backup '$OUT'"

# A backup that cannot be read back is not a backup.
log "vérification d'intégrité"
RESULT=$(sqlite3 "$OUT" 'PRAGMA integrity_check;')
[[ $RESULT == "ok" ]] || die "integrity_check a répondu: $RESULT"

gzip -f "$OUT"
OUT=$OUT.gz
log "ok: $(du -h "$OUT" | cut -f1)"

# --- off-box copy ---
if [[ -n ${B2_ACCOUNT_ID:-} && -n ${B2_APP_KEY:-} && -n ${B2_BUCKET:-} ]]; then
  log "envoi vers B2 ($B2_BUCKET)"
  rclone \
    --config /dev/null \
    --b2-account "$B2_ACCOUNT_ID" \
    --b2-key "$B2_APP_KEY" \
    copy "$OUT" ":b2:$B2_BUCKET/$SERVICE/" \
    --stats-one-line --stats 0 \
    || die "sync B2 échoué"
  log "B2 ok"
else
  log "B2 non configuré (B2_ACCOUNT_ID / B2_APP_KEY / B2_BUCKET absents) — copie locale seulement"
fi

# Prune only after the remote copy succeeded, so a failing sync never leaves us
# with neither a local nor a remote backup.
log "purge des sauvegardes de plus de $KEEP_DAYS jours"
find "$DIR" -name "$SERVICE-*.sqlite.gz" -mtime +$KEEP_DAYS -delete

log "terminé"
