#!/usr/bin/env bash
# Dijalankan di server oleh GitHub Actions via SSH.
# Usage: IMAGE_TAG=<sha> ./deploy.sh      (SKIP_MIGRATE=1 untuk rollback aplikasi saja)
#
# Backup: pg_dump HANYA dibuat bila ada migration baru, dan selalu MENIMPA
# backup sebelumnya (satu file: backups/<app>-pre-migration.sql.gz).
set -euo pipefail
cd "$(dirname "$0")"
source .env
export IMAGE_TAG APP_SLUG
COMPOSE="docker compose -f docker-compose.prod.yml"
BACKUP="backups/${APP_SLUG}-pre-migration.sql.gz"
mkdir -p backups

echo "==> Pull image $IMAGE_TAG"
$COMPOSE pull api web-app
$COMPOSE up -d db

if [ "${SKIP_MIGRATE:-0}" != "1" ] && $COMPOSE run --rm api /app/server migrate has-pending; then
  echo "==> Ada migration baru — backup database (menimpa backup sebelumnya)"
  # Tulis ke file sementara dulu: bila pg_dump gagal, backup lama tetap utuh.
  $COMPOSE exec -T db pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "${BACKUP}.tmp"
  test -s "${BACKUP}.tmp" || { rm -f "${BACKUP}.tmp"; echo "Backup gagal, deploy dihentikan"; exit 1; }
  mv -f "${BACKUP}.tmp" "$BACKUP"
  echo "    tersimpan: $BACKUP (sebelum ${IMAGE_TAG:0:7})"

  echo "==> Migration"
  $COMPOSE run --rm api /app/server migrate up
else
  echo "==> Tidak ada migration baru — backup & migration dilewati"
fi

echo "==> Start"
$COMPOSE up -d

echo "==> Health check"
for i in $(seq 1 20); do
  if $COMPOSE exec -T api wget -qO- http://localhost:8080/healthz >/dev/null 2>&1; then echo "OK"; exit 0; fi
  sleep 3
done
echo "Health check GAGAL — cek: $COMPOSE logs api"; exit 1
