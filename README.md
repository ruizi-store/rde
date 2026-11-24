# RDE (Ruizi Desktop Environment)

<p align="center">
  <a href="https://github.com/ruizi-store/rde"><img src="frontend/static/favicon.svg" alt="RDE" width="120" /></a>
</p>
<p align="center"><b>云桌面，让 Linux 服务器触手可及</b></p>
<p align="center"><i>Ruizi Desktop Environment — A Web-based Desktop for Linux Servers</i></p>
<p align="center">
  <a href="https://www.gnu.org/licenses/gpl-3.0.html"><img src="https://img.shields.io/github/license/ruizi-store/rde?color=%231890FF" alt="License: GPL v3"></a>
  <a href="https://github.com/ruizi-store/rde/releases"><img src="https://img.shields.io/github/v/release/ruizi-store/rde" alt="GitHub release"></a>
  <a href="https://github.com/ruizi-store/rde"><img src="https://img.shields.io/github/stars/ruizi-store/rde?color=%231890FF&style=flat-square" alt="Stars"></a>
  <a href="https://github.com/ruizi-store/rde/issues"><img src="https://img.shields.io/github/issues/ruizi-store/rde" alt="Issues"></a>
</p>
<p align="center">
  <a href="/README.md"><img alt="中文" src="https://img.shields.io/badge/中文-d9d9d9"></a>
</p>

------------------------------

RDE（瑞子云桌面）是一个开源的 Web 桌面环境，将 Linux 服务器变成功能丰富的云桌面。通过浏览器即可获得类 Windows 的操作体验，涵盖文件管理、Docker 应用、系统监控等功能。

- **Web 桌面体验**：类 Windows 的窗口管理、任务栏、右键菜单，在浏览器中获得桌面级操作体验。
- **文件管理**：功能完整的文件管理器，支持上传、下载、预览、分享，TUS 协议断点续传。
- **Docker 应用商店**：可视化管理容器和镜像，一键部署常用应用。
- **系统监控**：CPU、内存、磁盘、网络实时监控仪表盘。
- **文件同步**：Syncthing 多设备文件同步。
- **备份与恢复**：支持本地、WebDAV、S3、SFTP 等多种备份目标。
- **Web 终端**：浏览器内完整的 Linux 终端（含 SSH/SFTP 远程连接）。
- **AI 助手**：智能对话、语音交互、企业微信/Telegram 网关集成。
- **Docker 部署**：官方镜像 + docker compose 单容器交付，**默认免登录**（见 `docker/README.md`）。
- **离线资源**：支持将 Docker 镜像 tar、依赖 deb 等预先放入 `/usr/share/rde/offline`，断网环境优先本地加载。

> 路线：产品正转向"带电脑的 AI Bot"（每个 Bot 一台持久沙箱电脑，小团队经 IM 派活），详见 `docs/ROADMAP.md`。

## 快速开始

### Docker 部署（唯一官方形态）

```bash
git clone https://github.com/ruizi-store/rde.git
cd rde/docker && docker compose up -d
```

访问 `http://服务器IP:3080` **直接进入桌面，无需注册账号**（默认 `RDE_AUTH_MODE=none` 免登录单用户模式，适合内网/反向代理之后的自托管场景）。

需要账号体系时在 compose 里设置 `RDE_AUTH_MODE=local`，重启后走安装向导 + 登录。

### 开发环境

```bash
git clone https://github.com/ruizi-store/rde.git
cd rde

# 前后端一键启动（免登录模式）
make dev

# 或分别启动
cd backend && go run .        # 后端
cd frontend && pnpm dev       # 前端
```

### 构建

```bash
make docker  # 构建 Docker 镜像 (rde:<版本> + rde:latest)
```

## 项目架构

```
rde/
├── backend/                 # Go 后端 (Gin + GORM + SQLite)
│   ├── core/                #   核心框架（认证、配置、数据库、i18n、事件、模块注册）
│   ├── modules/             #   功能模块（ai/files/docker/terminal/backup/ssh/sync/...）
│   ├── model/               #   数据模型
│   └── pkg/                 #   公共工具包（含 container 容器探测）
├── frontend/                # SvelteKit 前端 (Svelte 5 + Tailwind CSS v4)
│   └── src/
│       ├── desktop/         #   桌面环境（窗口管理、任务栏）
│       ├── apps/            #   应用 UI
│       └── lib/             #   组件库
├── docker/                  # Docker 镜像交付（Dockerfile/compose/entrypoint）
└── docs/                    # 产品路线（ROADMAP.md）与版本历史（CHANGELOG.md）
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25、Gin、GORM、SQLite |
| 前端 | Svelte 5、SvelteKit、Tailwind CSS v4、Vite 7 |
| 容器 | Docker API |
| AI | OpenAI 兼容 API / Ollama |

## 参与贡献

欢迎各种形式的贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 发起 Pull Request

## Star History

<a href="https://star-history.com/#ruizi-store/rde&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=ruizi-store/rde&type=Date&theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=ruizi-store/rde&type=Date" />
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=ruizi-store/rde&type=Date" />
  </picture>
</a>

## 许可证

Copyright © 2024-2026 RDE Contributors

本项目采用 [GPL-3.0](https://www.gnu.org/licenses/gpl-3.0.html) 开源许可证。
