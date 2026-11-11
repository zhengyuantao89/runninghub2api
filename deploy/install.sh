#!/usr/bin/env bash
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/runninghub2api}"
SERVICE_NAME="runninghub2api"
UNIT_SRC="$(cd "$(dirname "$0")" && pwd)/runninghub2api.service"

if [[ $EUID -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi

id -u rh2api >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin rh2api

mkdir -p "$APP_DIR/data" "$APP_DIR/web"
install -m 0755 ./runninghub2api "$APP_DIR/runninghub2api"
if [[ -d ./web/dist ]]; then
  rm -rf "$APP_DIR/web/dist"
  cp -a ./web/dist "$APP_DIR/web/dist"
fi
# 已有 .env 不覆盖，避免部署把后台改过的管理员相关配置回写掉
if [[ -f ./.env ]]; then
  if [[ -f "$APP_DIR/.env" ]]; then
    echo "keep existing $APP_DIR/.env (not overwritten by release)"
  else
    install -m 0600 ./.env "$APP_DIR/.env"
  fi
fi

chown -R rh2api:rh2api "$APP_DIR"
chmod 0755 "$APP_DIR/runninghub2api"

install -m 0644 "$UNIT_SRC" /etc/systemd/system/${SERVICE_NAME}.service
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"
systemctl --no-pager --full status "$SERVICE_NAME" || true

echo
echo "health: curl -s http://127.0.0.1:8088/health"
