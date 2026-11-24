#!/bin/sh
# Bot 工作台入口：启动 KasmVNC（内置 web 客户端）+ openbox
#
# 端口约定：
#   6901  KasmVNC web（内置客户端 + websocket，控制面反向代理直连这里）
#   5901  传统 RFB（暂不使用）
#
# 认证约定：VNC 层无认证（SecurityTypes None）。安全边界来自
# 沙箱网络（rde-bot-net 隔离）+ 控制面代理鉴权（RDE JWT）。

set -e

RESOLUTION="${RDE_SCREEN_RESOLUTION:-1280x800}"
DISPLAY_NUM=":1"
WS_PORT=6901

Xkasmvnc "$DISPLAY_NUM" \
  -geometry "$RESOLUTION" \
  -depth 24 \
  -websocketPort "$WS_PORT" \
  -rfbport 5901 \
  -AlwaysShared \
  -SecurityTypes None \
  -DisableBasicAuth \
  -httpd /usr/share/kasmvnc/www \
  -interface 0.0.0.0 \
  -publicIP 127.0.0.1 \
  -BlacklistTimeout 0 \
  -FreeKeyMappings \
  -SendCutText \
  -AcceptCutText &

# 等待 X server 套接字就绪
i=0
while [ $i -lt 30 ]; do
  [ -S /tmp/.X11-unix/X1 ] && break
  sleep 1
  i=$((i+1))
done

export DISPLAY="$DISPLAY_NUM"

# openbox 作为前台进程（退出即容器退出）
exec openbox --replace
