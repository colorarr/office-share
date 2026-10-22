#!/usr/bin/env sh
set -eu
/usr/sbin/nginx -t
systemctl reload nginx
