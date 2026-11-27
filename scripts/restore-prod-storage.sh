#!/usr/bin/env bash
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SOURCE_STORAGE="$APP_DIR/storage"
TARGET_STORAGE="/var/lib/dockan/volumes/dockan-panel-data"
SERVICE_NAME="dockan-dockan-panel.service"
ROOTFS_STORAGE="/var/lib/dockan/images/dockan-panel_latest.dockan/rootfs/app/storage"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Erreur: lance ce script avec sudo:"
  echo "  sudo $0"
  exit 1
fi

if [[ ! -f "$SOURCE_STORAGE/auth-users.json" ]]; then
  echo "Erreur: $SOURCE_STORAGE/auth-users.json introuvable."
  exit 1
fi

stamp="$(date +%Y%m%d-%H%M%S)"
backup="${TARGET_STORAGE}.backup-${stamp}"

systemctl stop "$SERVICE_NAME" || true

if findmnt -R "$ROOTFS_STORAGE" >/dev/null 2>&1; then
  umount "$ROOTFS_STORAGE" || umount -l "$ROOTFS_STORAGE"
fi

if [[ -e "$TARGET_STORAGE" ]]; then
  mv "$TARGET_STORAGE" "$backup"
  echo "Sauvegarde prod: $backup"
fi

mkdir -p "$TARGET_STORAGE"
cp -a --no-preserve=context "$SOURCE_STORAGE/." "$TARGET_STORAGE/"
chown -R root:root "$TARGET_STORAGE"

if command -v restorecon >/dev/null 2>&1; then
  restorecon -R "$TARGET_STORAGE" || true
fi

systemctl start "$SERVICE_NAME"
systemctl --no-pager --full status "$SERVICE_NAME"

echo
echo "Restauration terminee. Ouvre: http://127.0.0.1:9090/"
