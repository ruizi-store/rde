#!/bin/bash
#
# 集成 EmulatorJS 到前端静态目录（构建期打包，支持离线）
# 从 frontend/node_modules/@emulatorjs 读取构建产物
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/.."
EMULATORJS_PKG="$FRONTEND_DIR/node_modules/@emulatorjs/emulatorjs/data"
EMULATORJS_DEST="$FRONTEND_DIR/static/emulatorjs"
VERSION_FILE="$EMULATORJS_DEST/version.json"

# 需要复制的核心（与 frontend/src/apps/retrogame/constants.ts PLATFORMS 对齐）
CORES=(
    "fceumm"            # NES / FC
    "snes9x"            # SNES / SFC
    "gambatte"          # GB / GBC
    "mgba"              # GBA
    "mupen64plus_next"  # N64
    "melonds"           # NDS
    "pcsx_rearmed"      # PlayStation
    "ppsspp"            # PSP
    "genesis_plus_gx"   # Sega Genesis / MD
    "yabause"           # Sega Saturn
    "fbneo"             # Arcade
)

EXPECTED_VERSION="4.2.3"

echo "=== EmulatorJS 集成脚本 ==="
echo "npm 包目录: $EMULATORJS_PKG"
echo "目标目录: $EMULATORJS_DEST"
echo ""

# 检查 npm 包是否已安装
if [[ ! -d "$EMULATORJS_PKG" ]]; then
    echo "错误: 未找到 @emulatorjs/emulatorjs npm 包"
    echo "请先运行: cd frontend && pnpm install"
    exit 1
fi

# 已构建且版本匹配则跳过（加速重复 build/dev）
if [[ -f "$EMULATORJS_DEST/emulator.min.js" && -f "$VERSION_FILE" ]]; then
    if grep -q "$EXPECTED_VERSION" "$VERSION_FILE" 2>/dev/null; then
        missing_core=0
        for core in "${CORES[@]}"; do
            if ! compgen -G "$EMULATORJS_DEST/cores/${core}*.data" > /dev/null; then
                missing_core=1
                break
            fi
        done
        if [[ "$missing_core" -eq 0 ]]; then
            echo "EmulatorJS $EXPECTED_VERSION 已就绪，跳过重建"
            du -sh "$EMULATORJS_DEST"
            exit 0
        fi
    fi
fi

# 从稳定版源码生成 minified 文件
echo ">>> 生成 minified 文件..."
MINIFY_TMP="$(mktemp -d)"
trap 'rm -rf "$MINIFY_TMP"' EXIT
mkdir -p "$MINIFY_TMP/src"
cp "$EMULATORJS_PKG/src/"*.js "$MINIFY_TMP/src/"
cp "$EMULATORJS_PKG/emulator.css" "$MINIFY_TMP/"

echo "  生成 emulator.min.js ..."
cd "$FRONTEND_DIR"
npx -y terser $MINIFY_TMP/src/*.js -o "$MINIFY_TMP/emulator.min.js" --compress --mangle 2>/dev/null || {
    echo "  terser 不可用，使用简单合并..."
    cat $MINIFY_TMP/src/*.js > "$MINIFY_TMP/emulator.min.js"
}
echo "  生成 emulator.min.css ..."
npx -y cleancss -o "$MINIFY_TMP/emulator.min.css" "$MINIFY_TMP/emulator.css" 2>/dev/null || {
    cp "$MINIFY_TMP/emulator.css" "$MINIFY_TMP/emulator.min.css"
}

# 创建目标目录
rm -rf "$EMULATORJS_DEST"
mkdir -p "$EMULATORJS_DEST/cores/reports"
mkdir -p "$EMULATORJS_DEST/src"
mkdir -p "$EMULATORJS_DEST/localization"
mkdir -p "$EMULATORJS_DEST/compression"

# 复制主文件
echo ">>> 复制主文件..."
cp "$EMULATORJS_PKG/loader.js" "$EMULATORJS_DEST/"
cp "$MINIFY_TMP/emulator.min.js" "$EMULATORJS_DEST/"
cp "$MINIFY_TMP/emulator.min.css" "$EMULATORJS_DEST/"
cp "$EMULATORJS_PKG/emulator.css" "$EMULATORJS_DEST/"
cp "$EMULATORJS_PKG/version.json" "$EMULATORJS_DEST/"

# 复制 src 目录
echo ">>> 复制 src 目录..."
cp "$EMULATORJS_PKG/src/"*.js "$EMULATORJS_DEST/src/"

# 复制 compression 目录
echo ">>> 复制 compression 目录..."
cp "$EMULATORJS_PKG/compression/"*.js "$EMULATORJS_DEST/compression/"
cp "$EMULATORJS_PKG/compression/"*.wasm "$EMULATORJS_DEST/compression/" 2>/dev/null || true

# 复制 localization 目录
echo ">>> 复制 localization 目录..."
cp "$EMULATORJS_PKG/localization/"*.json "$EMULATORJS_DEST/localization/"

# 复制核心 wasm 文件和 reports
echo ">>> 复制模拟器核心..."
for core in "${CORES[@]}"; do
    # 使用 createRequire 从 emulatorjs 包上下文解析 core 包路径（兼容 pnpm 严格模式）
    CORE_DIR=$(node -e "const{createRequire}=require('module');const r=createRequire(require.resolve('@emulatorjs/emulatorjs/package.json'));try{console.log(r.resolve('@emulatorjs/core-${core}/package.json').replace('/package.json',''))}catch{}" 2>/dev/null)
    if [[ -n "$CORE_DIR" && -d "$CORE_DIR" ]]; then
        for datafile in "$CORE_DIR/"*.data; do
            if [[ -f "$datafile" ]]; then
                fname=$(basename "$datafile")
                cp "$datafile" "$EMULATORJS_DEST/cores/"
                echo "  [复制] ${fname}"
            fi
        done
        if [[ -f "$CORE_DIR/reports/${core}.json" ]]; then
            cp "$CORE_DIR/reports/${core}.json" "$EMULATORJS_DEST/cores/reports/"
            echo "  [复制] reports/${core}.json"
        fi
    else
        echo "  [错误] ${core} - 未安装" >&2
        exit 1
    fi
done

echo ""
echo "=== 集成完成 ==="
echo ""
echo "文件大小统计:"
du -sh "$EMULATORJS_DEST"
du -sh "$EMULATORJS_DEST/cores" 2>/dev/null || true
du -sh "$EMULATORJS_DEST/src" 2>/dev/null || true
echo ""
