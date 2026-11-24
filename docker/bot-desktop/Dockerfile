# Bot 工作台沙箱镜像：Debian + KasmVNC + Chromium + openbox
#
# 构建（仓库根目录）：
#   sudo docker build -f docker/bot-desktop/Dockerfile -t rde/bot-desktop:latest . \
#     --build-arg HTTP_PROXY=... --build-arg HTTPS_PROXY=...
#
# 设计要点：
# - KasmVNC deb 来自 thirdparty/kasmvnc（离线资产，amd64/arm64 双架构）
# - /workspace 为 bot 的持久工作目录（由控制面 bind mount）
# - KasmVNC web 客户端监听 :6901，VNC 无认证（网络隔离 + 控制面代理鉴权）

FROM debian:trixie-slim

ARG TARGETARCH

# KasmVNC 离线 deb（按架构选择）
COPY thirdparty/kasmvnc/kasmvncserver_trixie_1.4.0_amd64.deb /tmp/kasmvnc-amd64.deb
COPY thirdparty/kasmvnc/kasmvncserver_trixie_1.4.0_arm64.deb /tmp/kasmvnc-arm64.deb

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl procps less vim git \
        chromium \
        openbox \
        xterm \
        fonts-noto-cjk \
        fonts-noto-color-emoji \
        python3 \
        libssl3 \
    && arch=$(echo "${TARGETARCH:-amd64}" | tr '[:upper:]' '[:lower:]') \
    && dpkg -i --force-depends /tmp/kasmvnc-${arch}.deb || true \
    && apt-get -f install -y --no-install-recommends \
    && rm -f /tmp/kasmvnc-*.deb \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /workspace \
    && useradd -m -s /bin/bash bot \
    && chown bot:bot /workspace \
    && which Xkasmvnc

# KasmVNC 启动参数全部走 entrypoint 的命令行（对齐旧 flatpak 模块验证过的用法）

COPY docker/bot-desktop/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER bot
WORKDIR /workspace

EXPOSE 6901

ENTRYPOINT ["/entrypoint.sh"]
