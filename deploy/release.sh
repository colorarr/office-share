#!/usr/bin/env bash
# Run on the server after uploading a trusted archive. Secrets are provisioned separately.
# Usage: sudo bash deploy/release.sh /tmp/office-share-release.tar.gz
set -euo pipefail
archive=${1:?Provide the absolute path of the release archive}
[[ "$EUID" -eq 0 ]] || { printf 'Run this script as root.\n' >&2; exit 1; }
[[ -f /etc/office-share.env ]] || { printf 'Provision /etc/office-share.env first.\n' >&2; exit 1; }
release="/opt/office-share/releases/$(date -u +%Y%m%dT%H%M%SZ)"
previous=$(readlink -f /opt/office-share/current 2>/dev/null || true)
mkdir -p "$release"
tar -xzf "$archive" -C "$release"
# macOS staging directories may be 0700; explicitly normalize release permissions.
find "$release" -type d -exec chmod 755 {} +
find "$release" -type f -exec chmod 644 {} +
chmod 755 "$release/office-share" "$release"/deploy/*.sh
chown -R root:root "$release"
ln -sfn "$release" /opt/office-share/current
install -m 644 "$release/deploy/office-share.service" /etc/systemd/system/office-share.service
systemctl daemon-reload
systemctl restart office-share
healthy=false
for attempt in $(seq 1 20); do
  if curl -fsS http://127.0.0.1:18787/api/health >/dev/null; then healthy=true; break; fi
  sleep 1
done
if [[ "$healthy" != true ]]; then
  if [[ -n "$previous" ]]; then
    ln -sfn "$previous" /opt/office-share/current
    systemctl restart office-share
  fi
  printf 'New release failed health checks; previous link restored when available.\n' >&2
  exit 1
fi
printf 'Healthy release: %s\n' "$release"
