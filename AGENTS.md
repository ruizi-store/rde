# Agent 指南 — RDE 开发环境

RDE（瑞子云桌面 / Ruizi Desktop Environment）是基于浏览器的 Linux 云桌面，后端为 **Go**，前端为 **SvelteKit**。

## 前置要求

| 工具 | 所需版本 |
|------|----------|
| Go | 1.25.5+（见 `backend/go.mod`） |
| Node.js | 20.19.1（Makefile 默认） |
| pnpm | 最新版 |

系统依赖：`sudo`（`make dev` 会以 root 跑后端）、`curl`、`make`。

## 一次性安装

```bash
# 安装 / 升级 Go、Node、pnpm
make setup
export PATH=/usr/local/go/bin:/usr/local/node/bin:$PATH

# 运行时目录（后端默认路径需要这些目录）
sudo mkdir -p /var/lib/rde/db /var/lib/rde/conf /var/log/rde /var/run/rde /etc/rde
sudo cp -n backend/conf/conf.conf.sample /etc/rde/rde.conf
sudo chmod -R a+rwX /var/lib/rde /var/log/rde /var/run/rde

# 依赖
cd frontend && pnpm install --config.dangerouslyAllowAllBuilds=true && cd ..
cd backend && go mod download && cd ..
```

说明：

- `make setup` 会把 Go/Node 装到 `/usr/local`，请优先使用该 PATH，而不是系统自带版本。
- 若国内镜像不可用，可改用官方源：
  - `go env -w GOPROXY=https://proxy.golang.org,direct`
  - `npm config set registry https://registry.npmjs.org`
- pnpm 10 可能跳过依赖的 build 脚本；按上面方式允许 `esbuild`（Vite）构建即可。

## 启动

```bash
make dev     # 后端 :3080，前端 :5175
make stop    # 停止两者
```

或分别启动：

```bash
# 后端（需要对 /var/lib/rde 有写权限；make dev 使用 sudo）
cd backend && sudo ./rde-backend   # 先执行: go build -o rde-backend .

# 前端（将 /api 与 /ws 代理到 :3080）
cd frontend && pnpm dev --port 5175
```

## 验证

- 前端界面：`http://localhost:5175/`（未初始化时显示安装向导）
- 后端健康检查：`curl -s http://localhost:3080/health`
- Setup API：`curl -s http://localhost:3080/api/v1/setup/status`
- 经 Vite 代理：`curl -s http://localhost:5175/api/v1/setup/status`

首次运行时 setup 未完成（`completed: false`），需通过向导创建管理员账号。

## 目录结构

```
backend/     Go API（Gin），模块在 backend/modules/
frontend/    SvelteKit + Tailwind v4 + Vite
debian/      DEB 打包配置
docker/      Docker 镜像交付（entrypoint、compose、部署文档）
docs/        产品路线（docs/ROADMAP.md：带电脑的 Bot 方向）
Makefile     setup / dev / stop / deb / docker
```

## Docker 形态（与 DEB 并存）

```bash
make docker    # 构建镜像 rde:<版本> + rde:latest（走 goproxy.cn）
cd docker && docker compose up -d   # 数据目录 /var/lib/rde，挂载 docker.sock
```

- 构建三阶段：node:20 → golang:1.25 → debian:bookworm-slim；前端构建产物在 `frontend/build/`。
- **模块开关**（`backend/core/bootstrap/bootstrap.go`）：环境变量 `RDE_MODULES`（白名单）/ `RDE_MODULES_DISABLED`（黑名单），控制 3 个可选模块（ssh/sync/cloudbackup）；核心 10 模块始终加载。
- **容器探测**（`backend/pkg/container`）：优先读 `RDE_CONTAINER=1`（镜像内已设置）；setup 检查跳过 systemd，system 模块禁用宿主机关机。
- **路径约定**：`/var/lib/rde` 宿主机与容器内必须同路径（docker-apps 生成的 compose bind mount 按宿主机解析）。
- daemon 无法直连 docker.io 时：预拉 `docker.m.daocloud.io/library/*` 基础镜像并 `docker tag` 成标准名。

## 常见问题

1. **Go 版本不对** — 系统自带 Go（如 1.22）无法编译本仓库；`make setup` 后请使用 `/usr/local/go/bin/go`。
2. **缺少数据目录** — 后端默认使用 `/var/lib/rde` 与 `/etc/rde/rde.conf`。
3. **端口不一致** — README 里生产/安装场景常见 `:80` / `:5173`；`make dev` 使用 **3080** / **5175**。
4. **可选服务缺失** — Docker、Flatpak、LibreTranslate、aria2 等未安装时可能打 warning，核心 UI/API 仍可正常工作。
