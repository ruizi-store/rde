#!/usr/bin/env bash
set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}>>>${NC} $*"; }
log_warn()  { echo -e "${YELLOW}>>>${NC} $*"; }
log_error() { echo -e "${RED}>>>${NC} $*" >&2; }

main() {
    log_info "Anbox Binder 模块卸载"
    
    # 检查 root 权限
    if [ "$EUID" -ne 0 ]; then
        log_error "此脚本需要 root 权限运行"
        exit 1
    fi
    
    # 卸载已加载的模块
    if lsmod | grep -q binder_linux; then
        log_info "卸载 binder_linux 模块..."
        rmmod binder_linux || log_warn "无法卸载模块，可能正在使用中"
    fi
    
    # 从 DKMS 移除
    if command -v dkms &>/dev/null; then
        if dkms status anbox-binder/1 &>/dev/null && [ -n "$(dkms status anbox-binder/1)" ]; then
            log_info "从 DKMS 移除 anbox-binder/1..."
            dkms remove anbox-binder/1 --all || log_warn "DKMS 移除失败"
        else
            log_info "DKMS 中未找到 anbox-binder/1"
        fi
    fi
    
    # 移除模块源码
    if [ -d /usr/src/anbox-binder-1 ]; then
        log_info "移除模块源码..."
        rm -rf /usr/src/anbox-binder-1
    fi
    
    # 移除配置文件
    log_info "移除配置文件..."
    rm -f /etc/modules-load.d/anbox.conf
    rm -f /lib/udev/rules.d/99-anbox.rules
    
    # 验证
    log_info "验证卸载..."
    local issues=0
    
    if lsmod | grep -q binder_linux; then
        log_warn "模块仍在加载中，请重启后验证"
        issues=1
    fi
    
    if [ -e /dev/binder ]; then
        log_warn "设备 /dev/binder 仍存在，请重启后验证"
        issues=1
    fi
    
    if [ $issues -eq 0 ]; then
        log_info "卸载完成!"
    else
        log_warn "卸载完成，但可能需要重启才能完全生效"
    fi
}

main "$@"
