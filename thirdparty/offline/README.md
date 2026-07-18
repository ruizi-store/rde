# RDE Offline Assets

离线优先资源目录。打包进 deb 后安装到 `/usr/share/rde/offline/`。

## 布局

```text
offline/
  manifest.yaml   # 资源索引
  images/         # docker save 产出的 *.tar / *.tar.gz
  debs/           # 可选本地 .deb
  drivers/        # 可选驱动/模块包
```

## 配置

在 `/etc/rde/rde.conf`：

```ini
[offline]
# 默认 /usr/share/rde/offline
Dir = /usr/share/rde/offline
```

运行时拉取 Docker 镜像、安装部分依赖时，会**先查本地离线包**，失败或不存在再走网络。

## 制作镜像离线包

```bash
mkdir -p images
docker pull redroid/redroid:14.0.0-latest
docker save -o images/redroid_redroid_14.0.0-latest.tar redroid/redroid:14.0.0-latest

docker pull tinylab/linux-lab
docker save -o images/tinylab_linux-lab.tar tinylab/linux-lab
```

然后在 `manifest.yaml` 的 `images:` 下登记逻辑名与路径。

## 说明

- 大文件建议用 Git LFS 或发布附属 offline bundle，不必全部提交进主仓库。
- KasmVNC Debian 13 包已单独放在 `thirdparty/kasmvnc/`（不走本目录）。
- EmulatorJS 由前端构建打进 `www/emulatorjs`。
