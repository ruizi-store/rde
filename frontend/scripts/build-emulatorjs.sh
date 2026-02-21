#!/bin/bash
#
# 集成 EmulatorJS 到前端静态目录
# 从 frontend/node_modules/@emulatorjs 读取构建产物
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/.."
EMULATORJS_PKG="$FRONTEND_DIR/node_modules/@emulatorjs/emulatorjs/data"
EMULATORJS_DEST="$FRONTEND_DIR/static/emulatorjs"

# 需要复制的核心
CORES=(
    "mgba"              # GBA
    "fceumm"            # NES
    "snes9x"            # SNES
    "gambatte"          # GB/GBC
    "mupen64plus_next"  # N64
    "melonds"           # NDS
    "genesis_plus_gx"   # Sega Genesis/MD
)

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

# 从稳定版 4.2.3 源码生成 minified 文件
echo ">>> 生成 minified 文件..."
MINIFY_TMP="$(mktemp -d)"
mkdir -p "$MINIFY_TMP/src"
cp "$EMULATORJS_PKG/src/"*.js "$MINIFY_TMP/src/"
cp "$EMULATORJS_PKG/emulator.css" "$MINIFY_TMP/"

# 使用 terser 和 clean-css 生成 minified 文件
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
        # 复制所有 wasm.data 变体（普通/legacy/thread/thread-legacy）
        for datafile in "$CORE_DIR/"*.data; do
            if [[ -f "$datafile" ]]; then
                fname=$(basename "$datafile")
                cp "$datafile" "$EMULATORJS_DEST/cores/"
                echo "  [复制] ${fname}"
            fi
        done
        # 复制 reports JSON
        if [[ -f "$CORE_DIR/reports/${core}.json" ]]; then
            cp "$CORE_DIR/reports/${core}.json" "$EMULATORJS_DEST/cores/reports/"
            echo "  [复制] reports/${core}.json"
        fi
    else
        echo "  [跳过] ${core} - 未安装"
    fi
done

echo ""
echo "=== 集成完成 ==="
echo ""

# 清理临时目录
rm -rf "$MINIFY_TMP"

echo "文件大小统计:"
du -sh "$EMULATORJS_DEST"
du -sh "$EMULATORJS_DEST/cores" 2>/dev/null || true
du -sh "$EMULATORJS_DEST/src" 2>/dev/null || true
echo ""
