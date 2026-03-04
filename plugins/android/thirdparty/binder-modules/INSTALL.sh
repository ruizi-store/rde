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

# 检查内核头文件
check_kernel_headers() {
    local kernel_version=$(uname -r)
    local headers_path="/lib/modules/$kernel_version/build"
    local source_path="/lib/modules/$kernel_version/source"
    
    if [ -d "$headers_path" ] || [ -d "$source_path" ]; then
        log_info "找到内核头文件: $headers_path"
        return 0
    fi
    
    log_error "未找到内核头文件!"
    log_error "内核版本: $kernel_version"
    log_error "期望路径: $headers_path 或 $source_path"
    echo ""
    log_warn "请安装内核头文件后重试:"
    
    # 检测发行版并给出建议
    if command -v apt-get &>/dev/null; then
        echo "  Debian/Ubuntu:"
        echo "    sudo apt-get update"
        echo "    sudo apt-get install linux-headers-$kernel_version"
        echo ""
        echo "  如果上述命令失败，尝试:"
        echo "    sudo apt-get install linux-headers-\$(uname -r | sed 's/[^-]*-[^-]*-//')"
        echo "    # 或安装通用头文件"
        echo "    sudo apt-get install linux-headers-amd64  # Debian"
        echo "    sudo apt-get install linux-headers-generic  # Ubuntu"
    elif command -v dnf &>/dev/null; then
        echo "  Fedora/RHEL:"
        echo "    sudo dnf install kernel-devel-$kernel_version"
    elif command -v pacman &>/dev/null; then
        echo "  Arch Linux:"
        echo "    sudo pacman -S linux-headers"
    fi
    
    echo ""
    log_warn "如果您使用自定义内核，请使用 --kernelsourcedir 选项:"
    echo "    sudo dkms install anbox-binder/1 --kernelsourcedir=/path/to/kernel/source"
    
    return 1
}

# 主安装逻辑
main() {
    log_info "Anbox Binder 模块安装"
    
    # 检查 root 权限
    if [ "$EUID" -ne 0 ]; then
        log_error "此脚本需要 root 权限运行"
        exit 1
    fi
    
    # 检查 dkms 是否安装
    if ! command -v dkms &>/dev/null; then
        log_error "未找到 dkms 命令，请先安装 dkms"
        log_info "Debian/Ubuntu: sudo apt-get install dkms build-essential"
        log_info "Fedora/RHEL:   sudo dnf install dkms kernel-devel"
        log_info "Arch Linux:    sudo pacman -S dkms"
        exit 1
    fi
    
    # 检查内核头文件
    if ! check_kernel_headers; then
        exit 1
    fi
    
    # 安装配置文件
    log_info "安装配置文件..."
    cp anbox.conf /etc/modules-load.d/
    # udev 规则复制到 /etc/udev/rules.d/ (优先) 和 /lib/udev/rules.d/
    cp 99-anbox.rules /etc/udev/rules.d/
    cp 99-anbox.rules /lib/udev/rules.d/
    
    # 清理已存在的 DKMS 模块（如果有）
    if dkms status anbox-binder/1 &>/dev/null; then
        log_info "检测到已存在的 anbox-binder/1 模块，先进行清理..."
        # 卸载已加载的模块
        if lsmod | grep -q binder_linux; then
            log_info "卸载已加载的 binder_linux 模块..."
            rmmod binder_linux 2>/dev/null || true
        fi
        # 从 DKMS 移除
        dkms remove anbox-binder/1 --all 2>/dev/null || true
    fi
    
    # 清理旧的源码目录
    if [ -d /usr/src/anbox-binder-1 ]; then
        log_info "清理旧的源码目录..."
        rm -rf /usr/src/anbox-binder-1
    fi
    
    # 复制模块源码到 /usr/src/
    log_info "复制模块源码到 /usr/src/anbox-binder-1..."
    cp -rT binder /usr/src/anbox-binder-1
    
    # 使用 dkms 构建和安装
    log_info "运行 DKMS 安装..."
    if ! dkms install anbox-binder/1; then
        log_error "DKMS 安装失败"
        log_warn "请检查上方的错误信息"
        exit 1
    fi
    
    # 加载模块
    log_info "加载模块..."
    modprobe binder_linux

    # 确保开机自动加载
    log_info "配置开机自动加载..."
    if [ -f /etc/modules-load.d/anbox.conf ]; then
        if grep -q "^binder_linux$" /etc/modules-load.d/anbox.conf; then
            log_info "binder_linux 已配置在 /etc/modules-load.d/anbox.conf 中"
        else
            echo "binder_linux" >> /etc/modules-load.d/anbox.conf
            log_info "已将 binder_linux 追加到 /etc/modules-load.d/anbox.conf"
        fi
    else
        echo "binder_linux" > /etc/modules-load.d/anbox.conf
        log_info "已创建 /etc/modules-load.d/anbox.conf"
    fi
    # 确保 systemd-modules-load 服务已启用（重启后自动加载模块）
    if command -v systemctl &>/dev/null; then
        systemctl enable systemd-modules-load.service 2>/dev/null || true
    fi

    # 验证
    log_info "验证安装..."
    if lsmod | grep -q binder_linux; then
        log_info "模块已加载: binder_linux"
    else
        log_warn "模块加载验证失败"
    fi
    
    if [ -e /dev/binder ]; then
        log_info "设备已创建: /dev/binder"
        ls -alh /dev/binder
    else
        log_warn "设备 /dev/binder 未创建"
    fi
    
    log_info "安装完成!"
}

main "$@"
