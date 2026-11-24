#!/bin/bash
# RDE 容器入口脚本
# 职责：准备运行时目录与配置文件（等价于 deb 包 postinst 的容器版），然后启动后端。
set -euo pipefail

DATA_DIR="${RDE_DATA_DIR:-/var/lib/rde}"
CONF_FILE="/etc/rde/rde.conf"
PORT="${RDE_PORT:-3080}"

# 1. 运行时目录（数据目录挂载卷时可能为空）
mkdir -p "$DATA_DIR/db" "$DATA_DIR/conf" "$DATA_DIR/www" \
    /etc/rde /var/log/rde /var/run/rde

# 前端静态文件：数据目录挂载卷（尤其 bind mount）会遮盖镜像内容，
# 首次启动时从镜像内置副本复制到 $DATA_DIR/www
if [ ! -f "$DATA_DIR/www/index.html" ]; then
    echo "[entrypoint] seeding frontend static files into $DATA_DIR/www"
    mkdir -p "$DATA_DIR/www"
    cp -a /usr/share/rde/www/. "$DATA_DIR/www/"
fi

# 2. 首次启动生成最小配置（JWT 密钥由后端自动生成并持久化到 $DATA_DIR/jwt_secret）
if [ ! -f "$CONF_FILE" ]; then
    cat > "$CONF_FILE" <<EOF
[app]
DBPath     = $DATA_DIR
LogPath    = /var/log/rde/
UserDataPath = $DATA_DIR/conf

[server]
HttpPort = $PORT
HttpHost = 0.0.0.0
RunMode  = release

[common]
RuntimePath = /var/run/rde
EOF
    echo "[entrypoint] created default config at $CONF_FILE (port $PORT)"
fi

# 3. 启动（-p 强制端口与环境变量一致；Setup 完成前 main.go 默认 3080）
echo "[entrypoint] starting rde (data=$DATA_DIR port=$PORT modules=${RDE_MODULES:-all}${RDE_MODULES_DISABLED:+ disabled=$RDE_MODULES_DISABLED})"
exec /usr/bin/rde -c "$CONF_FILE" -p "$PORT" -data "$DATA_DIR"
