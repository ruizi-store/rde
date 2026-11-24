# RDE Docker 部署指南

RDE 的容器化形态为：**单容器（控制面）+ 宿主机 Docker Socket**。RDE 不拥有整台机器，只管理挂载给它的数据目录与（通过 docker.sock）宿主机上的容器。

## 快速开始

```bash
# 构建镜像
docker build -t rde:latest .

# 启动（见 docker/docker-compose.yml）
cd docker && docker compose up -d

# 浏览器访问 http://<服务器IP>:3080 完成安装向导
```

国内网络构建加速（二选一）：

```bash
# 方式 A：HTTP 代理（基础镜像仍需可达 docker.io，或在 daemon 配置代理）
docker build \
  --build-arg HTTP_PROXY=http://proxy.example.com:3128 \
  --build-arg HTTPS_PROXY=http://proxy.example.com:3128 \
  -t rde:latest .

# 方式 B：镜像源（daemon 无法直连 docker.io 时，预拉基础镜像并重打标签）
docker pull docker.m.daocloud.io/library/node:20-bookworm-slim
docker pull docker.m.daocloud.io/library/golang:1.25-bookworm
docker pull docker.m.daocloud.io/library/debian:bookworm-slim
docker tag docker.m.daocloud.io/library/node:20-bookworm-slim node:20-bookworm-slim
docker tag docker.m.daocloud.io/library/golang:1.25-bookworm golang:1.25-bookworm
docker tag docker.m.daocloud.io/library/debian:bookworm-slim debian:bookworm-slim
docker build -t rde:latest .

# 依赖下载加速（可与上述方式组合）
docker build \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg NPM_REGISTRY=https://registry.npmmirror.com \
  -t rde:latest .
```

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `RDE_PORT` | `3080` | 容器内监听端口（对外端口由 `-p` 映射决定，不必改它） |
| `RDE_DATA_DIR` | `/var/lib/rde` | 数据目录（一般不动；两侧路径一致性要求见下） |
| `TZ` | `Asia/Shanghai` | 时区（替代 DEB 安装时的 timedatectl） |
| `RDE_MODULES` | 空 | 可选模块白名单，逗号分隔，如 `files,docker,terminal,ai` |
| `RDE_MODULES_DISABLED` | 空 | 可选模块黑名单，逗号分隔；与白名单同时设置时白名单优先 |

核心模块（setup/users/files/system/notification/sudo/terminal/docker/backup/ai）始终启用，不受开关影响。

## 模块开关

可选模块清单：`ssh sync cloudbackup`（其余模块为核心，始终启用）

| 模块 | 容器内可用性 | 说明 |
|---|---|---|
| `sync` | ⚠️ 精简镜像未带 syncthing | 需要扩展镜像或应用商店安装容器 |
| `ssh` | ✅ | 终端 SFTP/SSH 功能依赖此模块 |
| `cloudbackup` | ✅ | 纯网络逻辑 |

> 历史版本包含 photos/video/download/samba/retrogame/vm/android/flatpak/linuxlab/translate
> 等模块，已在 `feat/bot-direction` 分支移除（产品转向"带电脑的 Bot"，见 docs/ROADMAP.md）。

后端 `/api/v1/modules` 会返回实际注册的模块，前端据此渲染。

## 路径约定（重要）

`/var/lib/rde` 在宿主机与容器内**必须同路径**。原因：Docker 应用商店把生成的 `docker-compose.yml` 写在数据目录下、交给宿主机 Docker 执行，其中的 bind mount 路径按宿主机文件系统解析。两侧同路径可免去路径翻译（后续如需不同路径，将引入 `RDE_HOST_DATA_DIR` 映射）。

## 数据与升级

- 所有状态（SQLite、配置、JWT 密钥、应用数据）都在 `/var/lib/rde` 卷中。
- 升级 = 拉取新镜像并重建容器，数据无需迁移：
  ```bash
  docker compose pull && docker compose up -d
  ```
- 前端静态资源内置在镜像 `/usr/share/rde/www`，首次启动由 entrypoint 复制到 `$DATA_DIR/www`。

## 与 DEB 安装的关系

同一套代码，两种形态：

| | DEB | Docker |
|---|---|---|
| systemd 服务 | ✅ `rde.service` | ❌（容器即服务，`restart: unless-stopped`） |
| 用户管理 | 映射系统用户（useradd/sudo） | RDE 内部账号体系 |
| 关机/重启 | systemctl | 禁用/仅重启容器 |
| 网络配置 | netplan | 端口映射 |
| 模块裁剪 | 不可 | `RDE_MODULES*` 环境变量 |

## 安全须知

- 挂载 `docker.sock` ≈ 宿主机 root 权限。不要把 RDE 暴露到不受信网络；公网访问务必走反向代理 + TLS + 强认证。
- AI 网关（Telegram/企微等）在未配置 `AllowedUsers` 白名单时默认拒绝所有消息。
- `execute_script` 等工具仅应运行在 Bot 沙箱工作台中（规划中），控制面进程内执行仅为过渡方案。
