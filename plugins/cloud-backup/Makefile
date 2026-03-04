# RDE Plugin: 云备份
# 独立构建 Makefile (无前端)

MODULE_NAME := rde-cloud-backup
PLUGIN_ID := cloud-backup
PACKAGE_NAME := rde-cloud-backup
VERSION := $(shell head -1 debian/changelog | grep -oP '\(\K[^)]+' | cut -d'-' -f1)
ARCH := amd64
OUTPUT_DIR := outputs
PLUGIN_DIR := /var/lib/rde/plugins/$(PLUGIN_ID)
DEB_NAME := $(PACKAGE_NAME)_$(VERSION)_$(ARCH).deb

.PHONY: build deb clean help

build: ## 编译 Go 二进制
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=1 go build -ldflags "-s -w" -o $(OUTPUT_DIR)/$(MODULE_NAME) ./cmd/
	@echo "✓ Built $(MODULE_NAME)"

deb: build ## 构建 DEB 包 (无前端)
	@echo "打包 $(PACKAGE_NAME) DEB..."
	@mkdir -p $(OUTPUT_DIR)
	@TEMP_DIR=$$(mktemp -d) && \
	PKG_DIR="$$TEMP_DIR/$(PACKAGE_NAME)_$(VERSION)_$(ARCH)" && \
	mkdir -p "$$PKG_DIR$(PLUGIN_DIR)" && \
	mkdir -p "$$PKG_DIR/DEBIAN" && \
	cp $(OUTPUT_DIR)/$(MODULE_NAME) "$$PKG_DIR$(PLUGIN_DIR)/" && \
	cp manifest.json "$$PKG_DIR$(PLUGIN_DIR)/" && \
	chmod 755 "$$PKG_DIR$(PLUGIN_DIR)/$(MODULE_NAME)" && \
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

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
