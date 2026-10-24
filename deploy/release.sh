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
# Cover every failure after switching, including restart failures under set -e.
rollback() {
  status=$?
  trap - ERR
  if [[ -n "$previous" ]]; then
    if ln -sfn "$previous" /opt/office-share/current &&
       install -m 644 "$previous/deploy/office-share.service" /etc/systemd/system/office-share.service &&
       systemctl daemon-reload && systemctl restart office-share; then
      printf 'Previous release and service unit restored.\n' >&2
    else
      printf 'Rollback failed; check the service manually.\n' >&2
    fi
  fi
  printf 'Release failed.\n' >&2
  exit "$status"
}
trap rollback ERR
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
  printf 'New release failed health checks.\n' >&2
  false
fi
trap - ERR
printf 'Healthy release: %s\n' "$release"
