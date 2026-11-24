# =============================================================================
# RDE 镜像（控制面）
#
# 形态：单容器 + 挂载宿主机 /var/run/docker.sock（CasaOS/1Panel 同款做法）。
# 数据目录约定为 /var/lib/rde，且宿主机与容器内路径必须一致
# （docker-apps 生成的 compose bind mount 路径按宿主机解析，两侧同路径可免翻译）。
#
# 构建:
#   docker build -t rde:dev .
#   docker build --build-arg VERSION=0.2.4 --build-arg GOPROXY=https://goproxy.cn,direct -t rde:0.2.4 .
# =============================================================================

# ---------- 前端构建 ----------
FROM node:20-bookworm-slim AS frontend
WORKDIR /build

# 国内网络可加: --build-arg NPM_REGISTRY=https://registry.npmmirror.com
ARG NPM_REGISTRY=""
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi \
    && npm install -g pnpm@latest --fund=false --audit=false \
    && pnpm install --frozen-lockfile --config.dangerouslyAllowAllBuilds=true

COPY frontend/ .
# build-emulatorjs.sh 从 node_modules 复制模拟器内核（retrogame 模块）
RUN pnpm run build \
    && mkdir -p /out \
    && cp -r build/* /out/

# ---------- 后端构建 ----------
FROM golang:1.25-bookworm AS backend
WORKDIR /src

ARG GOPROXY=""
ARG VERSION=dev
ENV CGO_ENABLED=0
COPY backend/go.mod backend/go.sum ./
RUN if [ -n "$GOPROXY" ]; then go env -w GOPROXY="$GOPROXY"; fi \
    && go mod download

COPY backend/ .
RUN go build -tags=nomsgpack \
    -ldflags="-s -w -X 'github.com/ruizi-store/rde/backend/common.VERSION=${VERSION}'" \
    -o /out/rde .

# ---------- 运行时 ----------
FROM debian:bookworm-slim

# 基础依赖：bash(web 终端) / ca-certificates+tzdata(HTTPS/时区) / curl(健康检查)
# 注：aria2、syncthing、samba、ffmpeg、qemu 等可选服务依赖不在精简镜像中，
#     对应模块启动时仅告警；如需完整功能请在 compose 中挂载或扩展镜像。
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        bash ca-certificates tzdata curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend /out/rde /usr/bin/rde
# 前端静态资源放 /usr/share/rde/www（避免被挂载到 /var/lib/rde 的卷遮盖），
# entrypoint 首次启动时复制到 $DATA_DIR/www（后端从该目录提供 SPA 服务）
COPY --from=frontend /out /usr/share/rde/www
COPY docker/entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh \
    && mkdir -p /var/lib/rde /etc/rde /var/log/rde /var/run/rde

# 模块开关：
#   RDE_MODULES            白名单（仅启用列出的可选模块）
#   RDE_MODULES_DISABLED   黑名单（禁用列出的可选模块）
#   两者都为空 = 启用全部（与 DEB 安装行为一致）
# 容器形态标记（backend/pkg/container 读取；用于跳过 systemd 等宿主机专属逻辑）
ENV RDE_CONTAINER=1 \
    TZ=Asia/Shanghai \
    RDE_MODULES="" \
    RDE_MODULES_DISABLED=""

EXPOSE 3080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -sf http://127.0.0.1:3080/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
