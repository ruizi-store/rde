# RDE Plugin: Android 投屏
# 独立构建 Makefile

MODULE_NAME := rde-android
PLUGIN_ID := android
PACKAGE_NAME := rde-android
VERSION := $(shell head -1 debian/changelog | grep -oP '\(\K[^)]+' | cut -d'-' -f1)
ARCH := amd64
OUTPUT_DIR := outputs
FRONTEND_DIR := frontend
BACKEND_DIR := backend
PLUGIN_DIR := /var/lib/rde/plugins/$(PLUGIN_ID)
DEB_NAME := $(PACKAGE_NAME)_$(VERSION)_$(ARCH).deb

.PHONY: build build-frontend deb clean help

build: build-frontend ## 编译 Go 二进制（含嵌入前端）
	@mkdir -p $(OUTPUT_DIR)
	@# 将前端构建产物复制到 backend 内供 go:embed 使用
	@rm -rf $(BACKEND_DIR)/frontend_dist
	@cp -r $(FRONTEND_DIR)/dist $(BACKEND_DIR)/frontend_dist
	CGO_ENABLED=1 go build -ldflags "-s -w" -o $(OUTPUT_DIR)/$(MODULE_NAME) ./$(BACKEND_DIR)/cmd/
	@echo "✓ Built $(MODULE_NAME)"

build-frontend: ## 构建前端 SPA
	cd $(FRONTEND_DIR) && pnpm install --frozen-lockfile
	cd $(FRONTEND_DIR) && pnpm run build
	@echo "✓ Built frontend"

deb: build ## 构建 DEB 包
	@echo "打包 $(PACKAGE_NAME) DEB..."
	@mkdir -p $(OUTPUT_DIR)
	@TEMP_DIR=$$(mktemp -d) && \
	PKG_DIR="$$TEMP_DIR/$(PACKAGE_NAME)_$(VERSION)_$(ARCH)" && \
	mkdir -p "$$PKG_DIR$(PLUGIN_DIR)" && \
	mkdir -p "$$PKG_DIR$(PLUGIN_DIR)/binder-modules" && \
	mkdir -p "$$PKG_DIR/DEBIAN" && \
	cp $(OUTPUT_DIR)/$(MODULE_NAME) "$$PKG_DIR$(PLUGIN_DIR)/" && \
	cp manifest.json "$$PKG_DIR$(PLUGIN_DIR)/" && \
	chmod 755 "$$PKG_DIR$(PLUGIN_DIR)/$(MODULE_NAME)" && \
	cp -r thirdparty/binder-modules/binder "$$PKG_DIR$(PLUGIN_DIR)/binder-modules/" && \
	cp thirdparty/binder-modules/anbox.conf "$$PKG_DIR$(PLUGIN_DIR)/binder-modules/" && \
	cp thirdparty/binder-modules/99-anbox.rules "$$PKG_DIR$(PLUGIN_DIR)/binder-modules/" && \
	cp debian/control "$$PKG_DIR/DEBIAN/" && \
	sed -i "s/^Version:.*/Version: $(VERSION)/" "$$PKG_DIR/DEBIAN/control" && \
	INSTALLED_SIZE=$$(du -sk "$$PKG_DIR" | cut -f1) && \
	echo "Installed-Size: $$INSTALLED_SIZE" >> "$$PKG_DIR/DEBIAN/control" && \
	for f in postinst prerm postrm; do \
		[ -f debian/$$f ] && cp debian/$$f "$$PKG_DIR/DEBIAN/" && chmod 755 "$$PKG_DIR/DEBIAN/$$f"; \
	done && \
	dpkg-deb --build --root-owner-group "$$PKG_DIR" "$(OUTPUT_DIR)/$(DEB_NAME)" && \
	rm -rf "$$TEMP_DIR"
	@echo "✓ $(DEB_NAME) 已生成"

clean: ## 清理构建产物
	rm -rf $(OUTPUT_DIR)
	rm -rf $(FRONTEND_DIR)/dist
	rm -rf $(FRONTEND_DIR)/node_modules
	rm -rf $(BACKEND_DIR)/frontend_dist

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
