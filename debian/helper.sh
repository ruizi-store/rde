#!/bin/bash

# RDE Helper Script

# 获取系统信息
GetSysInfo() {
    if [ -s "/etc/os-release" ]; then
        . /etc/os-release
        echo "${PRETTY_NAME:-Unknown}"
    fi
    echo "Bit:$(getconf LONG_BIT) Mem:$(free -m | awk '/Mem:/{print $2}')M Core:$(nproc)"
    uname -a
}

# 获取网卡信息
GetNetCard() {
    if [ "$1" == "1" ]; then
        # 虚拟网卡
        ls /sys/devices/virtual/net 2>/dev/null || echo ""
    else
        # 物理网卡
        if [ -d "/sys/devices/virtual/net" ] && [ -d "/sys/class/net" ]; then
            VIRTUAL=$(ls /sys/devices/virtual/net/ 2>/dev/null)
            ls /sys/class/net/ 2>/dev/null | grep -v -w "$VIRTUAL" 2>/dev/null || ls /sys/class/net/ 2>/dev/null
        else
            ls /sys/class/net/ 2>/dev/null
        fi
    fi
}

# 获取时区
GetTimeZone() {
    timedatectl 2>/dev/null | grep "Time zone" | awk '{printf $3}' || cat /etc/timezone 2>/dev/null || echo "UTC"
}

# 查看网卡状态
CatNetCardState() {
    if [ -e "/sys/class/net/$1/operstate" ]; then
        cat "/sys/class/net/$1/operstate"
    else
        echo "unknown"
    fi
}

# 获取 Docker 根目录
GetDockerRootDir() {
    if command -v docker &>/dev/null; then
        docker info 2>/dev/null | grep 'Docker Root Dir' | awk -F ':' '{print $2}' | tr -d ' '
    else
        echo ""
    fi
}

# 获取设备树信息
GetDeviceTree() {
    if [ -f /proc/device-tree/model ]; then
        cat /proc/device-tree/model 2>/dev/null | tr -d '\0'
    else
        echo ""
    fi
}
