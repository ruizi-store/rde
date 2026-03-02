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
  <a href="/docs/README.en.md"><img alt="English" src="https://img.shields.io/badge/English-d9d9d9"></a>
</p>

------------------------------

RDE（瑞子云桌面）是一个开源的 Web 桌面环境，将 Linux 服务器变成功能丰富的云桌面。通过浏览器即可获得类 Windows 的操作体验，涵盖文件管理、Docker 应用、系统监控等功能。

- **Web 桌面体验**：类 Windows 的窗口管理、任务栏、右键菜单，在浏览器中获得桌面级操作体验。
- **文件管理**：功能完整的文件管理器，支持上传、下载、预览、分享，TUS 协议断点续传。
- **Docker 应用商店**：可视化管理容器和镜像，一键部署常用应用。
- **系统监控**：CPU、内存、磁盘、网络实时监控仪表盘。
- **多协议下载**：基于 aria2，支持 HTTP/BT/磁力链接下载管理。
- **Samba 共享**：一键配置局域网文件共享服务。
- **文件同步**：Syncthing 多设备文件同步。
- **备份与恢复**：支持本地、WebDAV、S3、SFTP 等多种备份目标。
- **Web 终端**：浏览器内完整的 Linux 终端。
- **插件系统**：通过插件扩展更多高级功能。

## 快速开始

### 在线安装

```bash
# Debian / Ubuntu
wget https://github.com/ruizi-store/rde/releases/latest/download/rde_amd64.deb
sudo apt install ./rde_amd64.deb
```

安装后访问 `http://服务器IP:80` 即可使用。

### 开发环境

```bash
git clone https://github.com/ruizi-store/rde.git
cd rde

# 前后端一键启动
make dev

# 或分别启动
cd backend && go run .        # 后端 :80
cd frontend && pnpm dev       # 前端 :5173
```

### 构建

```bash
make deb    # 构建 DEB 安装包
```

## 项目架构

```
rde/
├── backend/                 # Go 后端 (Gin + GORM + SQLite)
│   ├── core/                #   核心框架（认证、配置、数据库、i18n、事件、插件）
│   ├── modules/             #   功能模块
│   │   ├── files/           #     文件管理
│   │   ├── docker/          #     Docker 容器管理
│   │   ├── terminal/        #     Web 终端
│   │   ├── backup/          #     备份还原
│   │   ├── download/        #     下载管理
│   │   └── ...              #     更多模块
│   ├── model/               #   数据模型
│   └── pkg/                 #   公共工具包
├── frontend/                # SvelteKit 前端 (Svelte 5 + Tailwind CSS v4)
│   └── src/
│       ├── desktop/         #   桌面环境（窗口管理、任务栏）
│       ├── apps/            #   应用 UI
│       └── lib/             #   组件库
└── debian/                  # DEB 打包配置
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25、Gin、GORM、SQLite |
| 前端 | Svelte 5、SvelteKit、Tailwind CSS v4、Vite 7 |
| 容器 | Docker API |
| 下载 | aria2 RPC |
| 插件 | Unix Socket HTTP 通信、热插拔加载 |

## 参与贡献

欢迎各种形式的贡献！请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 发起 Pull Request

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=ruizi-store/rde&type=Date)](https://star-history.com/#ruizi-store/rde&Date)

## 许可证

Copyright © 2024-2026 RDE Contributors

本项目采用 [GPL-3.0](https://www.gnu.org/licenses/gpl-3.0.html) 开源许可证。
