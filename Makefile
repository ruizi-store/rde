# RDE Makefile

.PHONY: help dev stop deb deb-arm64 deb-all setup build-plugins build-plugin-ai \
       build-plugin-vm build-plugin-android build-plugin-cloud-backup clean-plugins

.DEFAULT_GOAL := help

# 颜色
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
RESET := \033[0m

# 版本号
VERSION := $(shell head -1 debian/changelog 2>/dev/null | grep -oP '\(\K[^)-]+' || echo "0.4.16")
ARCH ?= amd64
PACKAGE_NAME := rde

# 目录
ROOT_DIR := $(shell pwd)
BACKEND_DIR := $(ROOT_DIR)/backend
FRONTEND_DIR := $(ROOT_DIR)/frontend
OUTPUT_DIR := $(ROOT_DIR)/outputs
PLUGINS_DIR := $(ROOT_DIR)/plugins

# PID 文件
PID_DIR := $(ROOT_DIR)/.pids
BACKEND_PID := $(PID_DIR)/backend.pid
FRONTEND_PID := $(PID_DIR)/frontend.pid

# Go/Node 版本
GO_VERSION := 1.25.5
NODE_VERSION := 20.19.1

export PATH := $(PATH):/usr/local/go/bin:/usr/local/node/bin

# 部署配置 (按实际环境修改)
DEPLOY_HOST ?= 127.0.0.1
DEPLOY_PORT ?= 22
DEPLOY_USER ?= root

#==============================================================================
# 帮助
#==============================================================================

help:
	@echo ""
	@echo "$(CYAN)RDE 开发工具$(RESET)"
	@echo ""
	@echo "  make dev     - 启动开发环境 (前端 5175 + 后端 3080)"
	@echo "  make stop    - 停止开发服务器"
	@echo "  make deb     - 构建 DEB 安装包 (默认 amd64, 含插件)"
	@echo "  make deb-arm64 - 构建 ARM64 DEB 安装包"
	@echo "  make deb-all - 构建 amd64 + arm64 DEB 安装包"
	@echo "  make build-plugins   - 仅构建全部内置插件"
	@echo "  make clean-plugins   - 清理插件构建产物"
	@echo "  make setup   - 安装开发环境 (Go + Node + pnpm)"
	@echo ""
	@echo "$(YELLOW)版本: $(VERSION)$(RESET)"
	@echo ""

#==============================================================================
# 开发
#==============================================================================

dev:
	@echo "$(CYAN)启动开发环境...$(RESET)"
	@mkdir -p $(PID_DIR)
	@$(MAKE) -s stop 2>/dev/null || true
	@echo "$(GREEN)编译后端...$(RESET)"
	@cd $(BACKEND_DIR) && GOPROXY=https://goproxy.cn,direct go build -o rde-backend .
	@echo "$(GREEN)启动后端 (端口 3080)...$(RESET)"
	@cd $(BACKEND_DIR) && sudo nohup ./rde-backend > /tmp/rde-backend.log 2>&1 & echo $$! > $(BACKEND_PID)
	@sleep 1
	@echo "$(GREEN)启动前端 (端口 5175)...$(RESET)"
	@cd $(FRONTEND_DIR) && nohup pnpm dev --port 5175 > /tmp/rde-frontend.log 2>&1 & echo $$! > $(FRONTEND_PID)
	@sleep 2
	@echo ""
	@echo "$(GREEN)✓ 开发环境已启动$(RESET)"
	@echo "  后端: http://localhost:3080"
	@echo "  前端: http://localhost:5175"
	@echo ""
	@echo "$(YELLOW)使用 'make stop' 停止$(RESET)"

stop:
	@echo "$(CYAN)停止开发服务器...$(RESET)"
	@if [ -f $(BACKEND_PID) ]; then \
		PID=$$(cat $(BACKEND_PID)); \
		sudo kill $$PID 2>/dev/null || true; \
		rm -f $(BACKEND_PID); \
	fi
	@if [ -f $(FRONTEND_PID) ]; then \
		PID=$$(cat $(FRONTEND_PID)); \
		kill $$PID 2>/dev/null || true; \
		rm -f $(FRONTEND_PID); \
	fi
	@sudo pkill -f "rde-backend" 2>/dev/null || true
	@pkill -f "vite.*5175" 2>/dev/null || true
	@echo "$(GREEN)✓ 已停止$(RESET)"

#==============================================================================
# 构建
#==============================================================================

deb:
	@echo "$(CYAN)构建 DEB 包 (版本 $(VERSION), 架构 $(ARCH))...$(RESET)"
	@mkdir -p $(OUTPUT_DIR)
	dpkg-buildpackage -us -uc -b -d -a$(ARCH)
	@mv ../$(PACKAGE_NAME)_*.deb $(OUTPUT_DIR)/ 2>/dev/null || true
	@mv ../$(PACKAGE_NAME)_*.buildinfo $(OUTPUT_DIR)/ 2>/dev/null || true
	@mv ../$(PACKAGE_NAME)_*.changes $(OUTPUT_DIR)/ 2>/dev/null || true
	@echo "$(GREEN)✓ DEB 包已生成$(RESET)"
	@ls -lh $(OUTPUT_DIR)/$(PACKAGE_NAME)_*.deb

deb-arm64:
	@$(MAKE) deb ARCH=arm64

deb-all:
	@$(MAKE) deb ARCH=amd64
	@$(MAKE) deb ARCH=arm64

#==============================================================================
# 内置插件构建
#==============================================================================

build-plugin-ai: ## 构建 AI 插件
	@echo "$(GREEN)构建插件: AI...$(RESET)"
	@$(MAKE) -C $(PLUGINS_DIR)/ai build
	@echo "$(GREEN)✓ AI 插件已构建$(RESET)"

build-plugin-vm: ## 构建 VM 插件
	@echo "$(GREEN)构建插件: VM...$(RESET)"
	@$(MAKE) -C $(PLUGINS_DIR)/vm build build-frontend
	@echo "$(GREEN)✓ VM 插件已构建$(RESET)"

build-plugin-android: ## 构建 Android 插件
	@echo "$(GREEN)构建插件: Android...$(RESET)"
	@$(MAKE) -C $(PLUGINS_DIR)/android build
	@echo "$(GREEN)✓ Android 插件已构建$(RESET)"

build-plugin-cloud-backup: ## 构建云备份插件
	@echo "$(GREEN)构建插件: Cloud Backup...$(RESET)"
	@$(MAKE) -C $(PLUGINS_DIR)/cloud-backup build
	@echo "$(GREEN)✓ Cloud Backup 插件已构建$(RESET)"

build-plugins: build-plugin-ai build-plugin-vm build-plugin-android build-plugin-cloud-backup ## 构建全部内置插件
	@echo "$(GREEN)✓ 全部插件构建完成$(RESET)"

clean-plugins: ## 清理插件构建产物
	@for p in ai vm android cloud-backup; do \
		$(MAKE) -C $(PLUGINS_DIR)/$$p clean 2>/dev/null || true; \
	done
	@echo "$(GREEN)✓ 插件构建产物已清理$(RESET)"

#==============================================================================
# 环境安装
#==============================================================================

setup:
	@echo "$(CYAN)安装开发环境...$(RESET)"
	@# Go
	@if ! command -v go >/dev/null 2>&1; then \
		echo "$(GREEN)安装 Go $(GO_VERSION)...$(RESET)"; \
		HOST_ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/'); \
		curl -Lo /tmp/go.tar.gz https://go.dev/dl/go$(GO_VERSION).linux-$$HOST_ARCH.tar.gz; \
		sudo rm -rf /usr/local/go; \
		sudo tar -C /usr/local -xzf /tmp/go.tar.gz; \
		rm /tmp/go.tar.gz; \
	else \
		echo "$(YELLOW)Go 已安装: $$(go version)$(RESET)"; \
	fi
	@# Node.js
	@if ! command -v node >/dev/null 2>&1 || [ "$$(node -v | sed 's/^v//')" != "$(NODE_VERSION)" ]; then \
		echo "$(GREEN)安装 Node.js $(NODE_VERSION)...$(RESET)"; \
		HOST_ARCH=$$(uname -m | sed 's/x86_64/x64/;s/aarch64/arm64/'); \
		curl -Lo /tmp/node.tar.xz https://nodejs.org/dist/v$(NODE_VERSION)/node-v$(NODE_VERSION)-linux-$$HOST_ARCH.tar.xz; \
		sudo rm -rf /usr/local/node; \
		sudo mkdir -p /usr/local/node; \
		sudo tar -C /usr/local/node --strip-components=1 -xJf /tmp/node.tar.xz; \
		rm /tmp/node.tar.xz; \
	else \
		echo "$(YELLOW)Node.js 已安装: $$(node -v)$(RESET)"; \
	fi
	@# pnpm
	@if ! command -v pnpm >/dev/null 2>&1; then \
		echo "$(GREEN)安装 pnpm...$(RESET)"; \
		npm install -g pnpm; \
	else \
		echo "$(YELLOW)pnpm 已安装: $$(pnpm -v)$(RESET)"; \
	fi
	@# 镜像源
	@go env -w GOPROXY=https://goproxy.cn,direct 2>/dev/null || true
	@npm config set registry https://registry.npmmirror.com 2>/dev/null || true
	@# PATH
	@if ! grep -q "/usr/local/go/bin" ~/.bashrc 2>/dev/null; then \
		echo 'export PATH=$$PATH:/usr/local/go/bin:/usr/local/node/bin' >> ~/.bashrc; \
	fi
	@echo ""
	@echo "$(GREEN)✓ 开发环境安装完成$(RESET)"
	@echo "$(YELLOW)请执行: source ~/.bashrc$(RESET)"
