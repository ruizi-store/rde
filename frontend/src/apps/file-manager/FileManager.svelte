<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t, _ as translate, locale } from "svelte-i18n";
  import { get } from "svelte/store";
  import { windowManager } from "$desktop/stores/windows.svelte";
  import { fileService, type FileInfo, type BookmarkItem } from "$shared/services/files";
  import {
    ChunkedUploader,
    formatSize,
    formatTime,
    type UploadProgress,
    type UploadState,
  } from "$shared/services/chunked-upload";
  import { userStore } from "$shared/stores/user.svelte";
  import Icon from "@iconify/svelte";
  import LazyThumbnail from "$shared/components/LazyThumbnail.svelte";
  import { Checkbox, Button, Modal, Input, Alert, Progress } from "$shared/ui";

  let { windowId, initialPath }: { windowId: string; initialPath?: string } = $props();

  /* 状态 */
  let currentPath = $state("/");
  let files = $state<FileInfo[]>([]);
  let loading = $state(false);
  let error = $state("");
  let selectedPaths = $state<Set<string>>(new Set());
  let viewMode = $state<"grid" | "list" | "detail">("grid");
  let showHidden = $state(false);
  let searchKeyword = $state("");
  let sortBy = $state<"name" | "size" | "modified">("name");
  let sortAsc = $state(true);
  let sidebarCollapsed = $state(false);

  /* 剪贴板状态 */
  let clipboard = $state<{ paths: string[]; operation: "copy" | "cut" } | null>(null);

  /* 弹窗状态 */
  let showRenameModal = $state(false);
  let renameTarget = $state<FileInfo | null>(null);
  let newName = $state("");

  let showNewFolderModal = $state(false);
  let newFolderName = $state("");

  let showPreviewModal = $state(false);
  let previewFile = $state<FileInfo | null>(null);

  let showDeleteConfirm = $state(false);

  /* 移动到对话框状态 */
  let showMoveToModal = $state(false);
  let moveToPath = $state("/");
  let moveToFiles = $state<FileInfo[]>([]);
  let moveToLoading = $state(false);
  let moveToHistory = $state<string[]>([]);

  /* 上传状态 - 增强版 */
  interface UploadItem {
    id: string;
    file: File;
    relativePath?: string; // 用于文件夹上传，保留相对路径结构
    progress: number;
    status: "pending" | "uploading" | "paused" | "done" | "error";
    speed: number;
    remainingTime: number;
    uploader?: ChunkedUploader;
    error?: string;
  }
  let uploadQueue = $state<UploadItem[]>([]);
  let showUploadPanel = $state(false);
  let isDragOver = $state(false);
  let dragCounter = $state(0); // 用于正确处理嵌套元素的拖拽

  /* 内部文件拖拽状态 */
  let internalDragPaths = $state<string[]>([]); // 正在拖拽的文件路径
  let dropTargetFolder = $state<string | null>(null); // 拖拽目标文件夹

  /* 通知消息 */
  let notification = $state<{ type: "success" | "error" | "info"; message: string } | null>(null);



  /* 文件安装/运行状态（.deb / .flatpakref / .AppImage / .exe） */

  /* 管理员提权状态 */
  let elevated = $state(false);
  let elevateExpiresAt = $state<Date | null>(null);
  let elevateRemaining = $state(0); // 秒
  let elevateTimer: ReturnType<typeof setInterval> | null = null;
  let showElevateModal = $state(false);
  let elevatePassword = $state("");
  let elevateError = $state("");
  let elevateLoading = $state(false);
  let permissionDeniedPath = $state(""); // 权限不足时的路径（触发提权后自动重试）
  let needElevate = $state(false); // 当前是否因权限不足显示提权提示

  /* 快速访问位置 */
  let quickAccessDefaultRaw = $state<BookmarkItem[]>([
    { icon: "mdi:home", label: "fileManager.sidebar.homeDir", path: "/" },
  ]);
  // Use $derived to translate labels
  let quickAccessDefault = $derived(quickAccessDefaultRaw.map(item => ({
    ...item,
    label: item.label.startsWith("fileManager.") ? $t(item.label) : item.label
  })));
  let quickAccessSystem = $state<BookmarkItem[]>([]);

  /* 最近访问历史 */
  let history = $state<string[]>([]);
  let historyIndex = $state(-1);

  /* 地址栏编辑模式 */
  let addressBarEditing = $state(false);
  let addressBarInput = $state("");
  let addressBarInputRef = $state<HTMLInputElement | null>(null);

  /* 派生状态 - 排序和过滤后的文件列表 */
  let sortedFiles = $derived.by(() => {
    let result = [...files];

    // 搜索过滤
    if (searchKeyword) {
      const keyword = searchKeyword.toLowerCase();
      result = result.filter((f) => f.name.toLowerCase().includes(keyword));
    }

    // 排序 - 目录始终在前
    result.sort((a, b) => {
      if (a.is_dir !== b.is_dir) {
        return a.is_dir ? -1 : 1;
      }

      let cmp = 0;
      switch (sortBy) {
        case "name":
          cmp = a.name.localeCompare(b.name);
          break;
        case "size":
          cmp = (a.size || 0) - (b.size || 0);
          break;
        case "modified":
          cmp = new Date(a.modified).getTime() - new Date(b.modified).getTime();
          break;
      }
      return sortAsc ? cmp : -cmp;
    });

    return result;
  });

  /* 面包屑导航 */
  let breadcrumbs = $derived.by(() => {
    const parts = currentPath.split("/").filter(Boolean);
    const crumbs: { label: string; path: string }[] = [{ label: "/", path: "/" }];

    let accPath = "";
    for (const part of parts) {
      accPath += "/" + part;
      crumbs.push({ label: part, path: accPath });
    }

    return crumbs;
  });

  /* 计算选中文件的总大小 */
  let selectedSize = $derived.by(() => {
    let total = 0;
    for (const file of files) {
      if (selectedPaths.has(file.path) && !file.is_dir) {
        total += file.size || 0;
      }
    }
    return total;
  });

  /* 加载文件列表 */
  async function loadFiles() {
    loading = true;
    error = "";
    needElevate = false;

    try {
      const response: any = await fileService.list(currentPath, showHidden);
      if ((response.success === 200 || response.success === true) && response.data) {
        files = response.data.content || [];
        // 符号链接解析：如果后端返回了真实路径，更新当前路径和面包屑
        if (response.data.resolved_path && response.data.resolved_path !== currentPath) {
          currentPath = response.data.resolved_path;
          // 更新历史记录中的最后一项
          if (historyIndex >= 0 && historyIndex < history.length) {
            history[historyIndex] = currentPath;
            history = [...history];
          }
          windowManager.setTitle(windowId, $t("fileManager.titleWithPath", { values: { path: currentPath } }));
        }
      } else if (response.success === 403 && response.need_elevate) {
        // 权限不足，但可以通过管理员提权访问
        needElevate = true;
        permissionDeniedPath = currentPath;
        files = [];
        error = "";
      } else {
        error = response.message || $t("fileManager.error.loadFailed");
      }
    } catch (err) {
      error = err instanceof Error ? err.message : $t("fileManager.error.loadFailed");
    } finally {
      loading = false;
    }
  }

  /* 导航到路径 */
  function navigateTo(path: string, addToHistory = true) {
    if (addToHistory && currentPath !== path) {
      // 截断历史到当前位置并添加新路径
      history = [...history.slice(0, historyIndex + 1), path];
      historyIndex = history.length - 1;
    }

    currentPath = path;
    selectedPaths.clear();
    selectedPaths = new Set();
    windowManager.setTitle(windowId, $t("fileManager.titleWithPath", { values: { path } }));
    loadFiles();
  }

  /* 返回上级目录 */
  function goUp() {
    const parts = currentPath.split("/").filter(Boolean);
    if (parts.length > 0) {
      parts.pop();
      navigateTo("/" + parts.join("/"));
    }
  }

  /* 历史导航 */
  function goBack() {
    if (historyIndex > 0) {
      historyIndex--;
      navigateTo(history[historyIndex], false);
    }
  }

  function goForward() {
    if (historyIndex < history.length - 1) {
      historyIndex++;
      navigateTo(history[historyIndex], false);
    }
  }

  /* 刷新 */
  async function refresh() {
    await loadFiles();
    showNotification("info", $t("fileManager.notification.refreshed"));
  }

  /* 显示通知 */
  function showNotification(type: "success" | "error" | "info", message: string) {
    notification = { type, message };
    setTimeout(() => {
      notification = null;
    }, 3000);
  }

  /* 获取文件图标 */
  function getFileIcon(file: FileInfo): string {
    // 特殊文件夹图标
    if (file.is_dir) {
      const name = file.name.toLowerCase();
      const folderIcons: Record<string, string> = {
        "node_modules": "mdi:folder-home",
        ".git": "mdi:git",
        ".github": "mdi:github",
        ".vscode": "mdi:microsoft-visual-studio-code",
        "src": "mdi:folder-cog",
        "lib": "mdi:folder-star",
        "dist": "mdi:folder-upload",
        "build": "mdi:folder-upload",
        "public": "mdi:folder-open",
        "assets": "mdi:folder-image",
        "images": "mdi:folder-image",
        "img": "mdi:folder-image",
        "docs": "mdi:folder-text",
        "test": "mdi:folder-search",
        "tests": "mdi:folder-search",
        "__tests__": "mdi:folder-search",
        "config": "mdi:folder-cog",
        "scripts": "mdi:folder-play",
        "bin": "mdi:folder-play",
        "vendor": "mdi:folder-download",
        "packages": "mdi:folder-multiple",
        "components": "mdi:folder-star",
        "pages": "mdi:folder-file",
        "api": "mdi:folder-network",
        "styles": "mdi:folder-heart",
        "downloads": "mdi:folder-download",
        "desktop": "mdi:folder-home-outline",
        "documents": "mdi:folder-file",
        "pictures": "mdi:folder-image",
        "music": "mdi:folder-music",
        "videos": "mdi:folder-play",
      };
      return folderIcons[name] || "mdi:folder";
    }

    const ext = getFileExtension(file);
    const name = file.name.toLowerCase();

    // 特殊文件名图标
    const specialFiles: Record<string, string> = {
      "dockerfile": "mdi:docker",
      "docker-compose.yml": "mdi:docker",
      "docker-compose.yaml": "mdi:docker",
      ".dockerignore": "mdi:docker",
      "makefile": "mdi:cog",
      ".gitignore": "mdi:git",
      ".gitattributes": "mdi:git",
      ".env": "mdi:cog",
      ".env.local": "mdi:cog",
      ".env.development": "mdi:cog",
      ".env.production": "mdi:cog",
      "package.json": "mdi:nodejs",
      "package-lock.json": "mdi:nodejs",
      "pnpm-lock.yaml": "mdi:nodejs",
      "yarn.lock": "mdi:nodejs",
      "tsconfig.json": "mdi:language-typescript",
      "vite.config.ts": "simple-icons:vite",
      "vite.config.js": "simple-icons:vite",
      "webpack.config.js": "mdi:webpack",
      "rollup.config.js": "mdi:cog",
      "readme.md": "mdi:book-open-variant",
      "readme": "mdi:book-open-variant",
      "license": "mdi:license",
      "license.md": "mdi:license",
      "changelog.md": "mdi:history",
      "contributing.md": "mdi:handshake",
      "svelte.config.js": "mdi:svelte",
      "eslint.config.js": "mdi:eslint",
      ".eslintrc": "mdi:eslint",
      ".prettierrc": "mdi:code-braces",
      "go.mod": "mdi:language-go",
      "go.sum": "mdi:language-go",
      "cargo.toml": "mdi:language-rust",
      "requirements.txt": "mdi:language-python",
      "pipfile": "mdi:language-python",
    };
    if (specialFiles[name]) return specialFiles[name];

    const iconMap: Record<string, string> = {
      // 图片
      jpg: "mdi:file-image",
      jpeg: "mdi:file-image",
      png: "mdi:file-image",
      gif: "mdi:file-image",
      webp: "mdi:file-image",
      svg: "mdi:file-image",
      bmp: "mdi:file-image",
      ico: "mdi:file-image",
      tiff: "mdi:file-image",
      heic: "mdi:file-image",
      heif: "mdi:file-image",
      raw: "mdi:file-image",
      psd: "mdi:adobe-photoshop",
      ai: "mdi:adobe-illustrator",
      // 视频
      mp4: "mdi:file-video",
      mkv: "mdi:file-video",
      avi: "mdi:file-video",
      mov: "mdi:file-video",
      wmv: "mdi:file-video",
      webm: "mdi:file-video",
      flv: "mdi:file-video",
      m4v: "mdi:file-video",
      "3gp": "mdi:file-video",
      ts: "mdi:file-video",
      m2ts: "mdi:file-video",
      // 音频
      mp3: "mdi:file-music",
      wav: "mdi:file-music",
      flac: "mdi:file-music",
      ogg: "mdi:file-music",
      aac: "mdi:file-music",
      m4a: "mdi:file-music",
      wma: "mdi:file-music",
      opus: "mdi:file-music",
      mid: "mdi:file-music",
      midi: "mdi:file-music",
      // 文档
      pdf: "mdi:file-pdf-box",
      doc: "mdi:file-word",
      docx: "mdi:file-word",
      xls: "mdi:file-excel",
      xlsx: "mdi:file-excel",
      ppt: "mdi:file-powerpoint",
      pptx: "mdi:file-powerpoint",
      txt: "mdi:file-document",
      md: "mdi:language-markdown",
      rtf: "mdi:file-document",
      odt: "mdi:file-document",
      ods: "mdi:file-table",
      odp: "mdi:file-presentation-box",
      epub: "mdi:book-open-variant",
      mobi: "mdi:book-open-variant",
      csv: "mdi:file-delimited",
      // 代码 - JavaScript/TypeScript
      js: "mdi:language-javascript",
      mjs: "mdi:language-javascript",
      cjs: "mdi:language-javascript",
      jsx: "mdi:react",
      tsx: "mdi:react",
      // ts 扩展名被视频占用，通过特殊文件处理
      // 代码 - 其他语言
      py: "mdi:language-python",
      go: "mdi:language-go",
      java: "mdi:language-java",
      kt: "mdi:language-kotlin",
      swift: "mdi:language-swift",
      c: "mdi:language-c",
      cpp: "mdi:language-cpp",
      h: "mdi:language-c",
      hpp: "mdi:language-cpp",
      cs: "mdi:language-csharp",
      rs: "mdi:language-rust",
      rb: "mdi:language-ruby",
      php: "mdi:language-php",
      lua: "mdi:language-lua",
      r: "mdi:language-r",
      dart: "mdi:flutter",
      scala: "mdi:language-scala",
      // Web 技术
      html: "mdi:language-html5",
      htm: "mdi:language-html5",
      css: "mdi:language-css3",
      scss: "mdi:sass",
      sass: "mdi:sass",
      less: "mdi:language-css3",
      vue: "mdi:vuejs",
      svelte: "mdi:svelte",
      // 配置/数据
      json: "mdi:code-json",
      jsonc: "mdi:code-json",
      json5: "mdi:code-json",
      xml: "mdi:file-xml-box",
      yaml: "mdi:file-code",
      yml: "mdi:file-code",
      toml: "mdi:file-code",
      ini: "mdi:file-cog",
      conf: "mdi:file-cog",
      cfg: "mdi:file-cog",
      // 脚本
      sh: "mdi:console",
      bash: "mdi:console",
      zsh: "mdi:console",
      fish: "mdi:console",
      ps1: "mdi:powershell",
      bat: "mdi:console",
      cmd: "mdi:console",
      // 数据库
      sql: "mdi:database",
      sqlite: "mdi:database",
      db: "mdi:database",
      // 压缩包
      zip: "mdi:folder-zip",
      rar: "mdi:folder-zip",
      tar: "mdi:folder-zip",
      gz: "mdi:folder-zip",
      "7z": "mdi:folder-zip",
      xz: "mdi:folder-zip",
      bz2: "mdi:folder-zip",
      tgz: "mdi:folder-zip",
      // 安装包/二进制
      exe: "mdi:application",
      msi: "mdi:application",
      apk: "mdi:android",
      aab: "mdi:android",
      ipa: "mdi:apple",
      app: "mdi:apple",
      iso: "mdi:disc",
      img: "mdi:disc",
      dmg: "mdi:apple",
      deb: "mdi:debian",
      rpm: "mdi:package",
      pkg: "mdi:package",
      snap: "mdi:package",
      flatpak: "mdi:package",
      appimage: "mdi:package",
      // 字体
      ttf: "mdi:format-font",
      otf: "mdi:format-font",
      woff: "mdi:format-font",
      woff2: "mdi:format-font",
      eot: "mdi:format-font",
      // 证书/密钥
      pem: "mdi:key",
      key: "mdi:key",
      crt: "mdi:certificate",
      cer: "mdi:certificate",
      pfx: "mdi:key",
      p12: "mdi:key",
      // 其他
      log: "mdi:file-document-outline",
      lock: "mdi:lock",
      bak: "mdi:backup-restore",
      tmp: "mdi:file-clock",
      swp: "mdi:file-clock",
    };

    return iconMap[ext] || "mdi:file";
  }

  /* 获取文件颜色 */
  function getFileColor(file: FileInfo): string {
    // 特殊文件夹颜色
    if (file.is_dir) {
      const name = file.name.toLowerCase();
      const folderColors: Record<string, string> = {
        "node_modules": "#7CB342",
        ".git": "#F4511E",
        ".github": "#333333",
        ".vscode": "#0078D7",
        "src": "#42A5F5",
        "dist": "#66BB6A",
        "build": "#66BB6A",
      };
      return folderColors[name] || "#FFA726";
    }

    const ext = getFileExtension(file);
    const name = file.name.toLowerCase();

    // 特殊文件颜色
    if (name === "dockerfile" || name.startsWith("docker-compose")) return "#2496ED";
    if (name === "makefile") return "#6D4C41";
    if (name.endsWith(".gitignore") || name.endsWith(".gitattributes")) return "#F4511E";

    // 图片 - 青色系
    if (["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico", "tiff", "heic", "heif", "raw"].includes(ext)) return "#26A69A";
    if (["psd"].includes(ext)) return "#31A8FF"; // Photoshop 蓝
    if (["ai"].includes(ext)) return "#FF9A00"; // Illustrator 橙

    // 视频 - 红色系
    if (["mp4", "mkv", "avi", "mov", "wmv", "webm", "flv", "m4v", "3gp", "ts", "m2ts"].includes(ext)) return "#EF5350";

    // 音频 - 紫色系
    if (["mp3", "wav", "flac", "ogg", "aac", "m4a", "wma", "opus", "mid", "midi"].includes(ext)) return "#AB47BC";

    // 文档
    if (["pdf"].includes(ext)) return "#F44336";
    if (["doc", "docx", "odt"].includes(ext)) return "#2196F3";
    if (["xls", "xlsx", "ods", "csv"].includes(ext)) return "#4CAF50";
    if (["ppt", "pptx", "odp"].includes(ext)) return "#FF9800";
    if (["txt", "rtf", "md"].includes(ext)) return "#607D8B";
    if (["epub", "mobi"].includes(ext)) return "#795548";

    // 代码 - 按语言区分
    if (["js", "mjs", "cjs"].includes(ext)) return "#F7DF1E"; // JavaScript 黄
    if (["jsx", "tsx"].includes(ext)) return "#61DAFB"; // React 蓝
    if (["py"].includes(ext)) return "#3776AB"; // Python 蓝
    if (["go"].includes(ext)) return "#00ADD8"; // Go 蓝
    if (["java", "kt"].includes(ext)) return "#B07219"; // Java 棕
    if (["c", "h"].includes(ext)) return "#555555"; // C 灰
    if (["cpp", "hpp"].includes(ext)) return "#F34B7D"; // C++ 粉
    if (["cs"].includes(ext)) return "#178600"; // C# 绿
    if (["rs"].includes(ext)) return "#DEA584"; // Rust 橙
    if (["rb"].includes(ext)) return "#CC342D"; // Ruby 红
    if (["php"].includes(ext)) return "#4F5D95"; // PHP 紫蓝
    if (["swift"].includes(ext)) return "#F05138"; // Swift 橙红
    if (["dart"].includes(ext)) return "#00B4AB"; // Dart 青

    // Web 技术
    if (["html", "htm"].includes(ext)) return "#E34C26"; // HTML 橙
    if (["css", "scss", "sass", "less"].includes(ext)) return "#264DE4"; // CSS 蓝
    if (["vue"].includes(ext)) return "#42B883"; // Vue 绿
    if (["svelte"].includes(ext)) return "#FF3E00"; // Svelte 橙

    // 配置/数据
    if (["json", "jsonc", "json5"].includes(ext)) return "#CBCB41";
    if (["xml"].includes(ext)) return "#E37933";
    if (["yaml", "yml", "toml"].includes(ext)) return "#6D8086";

    // 脚本
    if (["sh", "bash", "zsh", "fish"].includes(ext)) return "#4EAA25";
    if (["ps1"].includes(ext)) return "#012456";

    // 数据库
    if (["sql", "sqlite", "db"].includes(ext)) return "#336791";

    // 压缩包 - 棕色系
    if (["zip", "rar", "tar", "gz", "7z", "xz", "bz2", "tgz"].includes(ext)) return "#8D6E63";

    // 安装包
    if (["exe", "msi"].includes(ext)) return "#00A4EF";
    if (["apk", "aab"].includes(ext)) return "#3DDC84"; // Android 绿
    if (["ipa", "app", "dmg"].includes(ext)) return "#A2AAAD"; // Apple 灰
    if (["deb"].includes(ext)) return "#A80030"; // Debian 红
    if (["rpm"].includes(ext)) return "#EE0000"; // Red Hat 红
    if (["iso", "img"].includes(ext)) return "#607D8B";

    // 字体
    if (["ttf", "otf", "woff", "woff2", "eot"].includes(ext)) return "#E91E63";

    // 证书/密钥
    if (["pem", "key", "crt", "cer", "pfx", "p12"].includes(ext)) return "#FFD600";

    // 其他
    if (["log"].includes(ext)) return "#9E9E9E";
    if (["lock"].includes(ext)) return "#FF5722";
    if (["bak", "tmp", "swp"].includes(ext)) return "#BDBDBD";

    return "#78909C";
  }

  /* 格式化日期 */
  function formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    // 今天
    if (diff < 24 * 60 * 60 * 1000 && date.getDate() === now.getDate()) {
      return $t("fileManager.date.today") + " " + date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
    }

    // 昨天
    const yesterday = new Date(now);
    yesterday.setDate(yesterday.getDate() - 1);
    if (date.getDate() === yesterday.getDate()) {
      return $t("fileManager.date.yesterday") + " " + date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
    }

    return date.toLocaleDateString(undefined, {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  /* 点击文件 */
  function handleItemClick(file: FileInfo, e: MouseEvent) {
    if (e.ctrlKey || e.metaKey) {
      // 多选
      if (selectedPaths.has(file.path)) {
        selectedPaths.delete(file.path);
      } else {
        selectedPaths.add(file.path);
      }
      selectedPaths = new Set(selectedPaths);
    } else if (e.shiftKey && selectedPaths.size > 0) {
      // 范围选择
      const lastSelected = Array.from(selectedPaths).pop();
      if (lastSelected) {
        const lastIdx = sortedFiles.findIndex((f) => f.path === lastSelected);
        const currentIdx = sortedFiles.findIndex((f) => f.path === file.path);
        const [start, end] = lastIdx < currentIdx ? [lastIdx, currentIdx] : [currentIdx, lastIdx];

        for (let i = start; i <= end; i++) {
          selectedPaths.add(sortedFiles[i].path);
        }
        selectedPaths = new Set(selectedPaths);
      }
    } else {
      // 单选
      selectedPaths = new Set([file.path]);
    }
  }

  /* 从文件名获取扩展名 */
  function getExtension(filename: string): string {
    const lastDot = filename.lastIndexOf(".");
    if (lastDot === -1 || lastDot === 0) return "";
    return filename.substring(lastDot + 1).toLowerCase();
  }

  /* 获取文件类型分类 */
  function getFileCategory(file: FileInfo): "image" | "video" | "audio" | "text" | "pdf" | "other" {
    // 优先使用 extension 字段，否则从文件名提取
    const ext = file.extension?.toLowerCase() || getExtension(file.name);

    if (["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp", "ico"].includes(ext)) {
      return "image";
    }
    if (["mp4", "webm", "mkv", "avi", "mov", "wmv", "flv", "m4v"].includes(ext)) {
      return "video";
    }
    if (["mp3", "wav", "ogg", "flac", "aac", "m4a", "wma"].includes(ext)) {
      return "audio";
    }
    if (
      [
        "txt",
        "md",
        "json",
        "js",
        "ts",
        "css",
        "html",
        "xml",
        "yaml",
        "yml",
        "sh",
        "bash",
        "py",
        "go",
        "java",
        "c",
        "cpp",
        "h",
        "hpp",
        "rs",
        "rb",
        "php",
        "sql",
        "conf",
        "ini",
        "log",
        "csv",
        "svelte",
        "vue",
        "jsx",
        "tsx",
      ].includes(ext)
    ) {
      return "text";
    }
    if (ext === "pdf") {
      return "pdf";
    }
    return "other";
  }

  /* 双击文件 */
  async function handleItemDoubleClick(file: FileInfo) {
    if (file.is_dir) {
      navigateTo(file.path);
    } else {
      // 打开预览
      const category = getFileCategory(file);

      // 视频文件使用视频播放器打开
      if (category === "video") {
        await windowManager.open("video", { filePath: file.path });
        return;
      }

      // 音频文件使用音乐播放器打开
      if (category === "audio") {
        await windowManager.open("music", { filePath: file.path });
        return;
      }

      if (category !== "other") {
        previewFile = file;
        showPreviewModal = true;

        // 如果是文本文件，加载内容
        if (category === "text") {
          loadFileContent(file);
        }
      } else {
        // 不支持预览的文件类型，提示用户
        showNotification("info", $t("fileManager.notification.previewNotSupported"));
      }
    }
  }

  /* 加载文件内容 */
  let fileContent = $state("");
  let fileContentLoading = $state(false);
  let fileContentEdited = $state(false);

  async function loadFileContent(file: FileInfo) {
    fileContentLoading = true;
    fileContent = "";
    fileContentEdited = false;

    try {
      const response = await fileService.getContent(file.path);
      if (response.success && response.data) {
        fileContent = response.data.content;
      } else {
        fileContent = $t("fileManager.notification.loadContentFailed");
      }
    } catch {
      fileContent = $t("fileManager.notification.loadFailed");
    } finally {
      fileContentLoading = false;
    }
  }

  /* 保存文件内容 */
  async function saveFileContent() {
    if (!previewFile) return;

    try {
      const response: any = await fileService.saveContent(previewFile.path, fileContent);
      if (response.success === 200 || response.success === true) {
        fileContentEdited = false;
        showNotification("success", $t("fileManager.notification.saveSuccess"));
      } else {
        showNotification("error", response.message || $t("fileManager.notification.saveFailed"));
      }
    } catch {
      showNotification("error", $t("fileManager.notification.saveFailed"));
    }
  }

  /* 全选 */
  function selectAll() {
    selectedPaths = new Set(sortedFiles.map((f) => f.path));
  }

  /* 取消选择 */
  function deselectAll() {
    selectedPaths = new Set();
  }

  /* 创建新文件夹 */
  async function createFolder() {
    if (!newFolderName.trim()) return;

    const folderName = newFolderName.trim();
    const targetPath = currentPath;

    try {
      const response: any = await fileService.createDir(targetPath, folderName);
      
      if (response.success === 200 || response.success === true) {
        showNewFolderModal = false;
        newFolderName = "";
        // 强制刷新文件列表
        files = [];
        await loadFiles();
        showNotification("success", $t("fileManager.notification.folderCreated"));
      } else if (response.success === 403 && response.need_elevate) {
        // 权限不足，需要管理员提权
        showNewFolderModal = false;
        permissionDeniedPath = targetPath;
        showElevateModal = true;
        showNotification("error", $t("fileManager.notification.permissionDenied"));
      } else {
        showNotification("error", response.message || $t("fileManager.notification.createFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.createFailed"));
    }
  }

  /* 重命名 */
  async function renameFile() {
    if (!renameTarget || !newName.trim()) return;

    try {
      const response: any = await fileService.rename(renameTarget.path, newName.trim());
      if (response.success === 200 || response.success === true) {
        showRenameModal = false;
        renameTarget = null;
        newName = "";
        refresh();
        showNotification("success", $t("fileManager.notification.renameSuccess"));
      } else {
        showNotification("error", response.message || $t("fileManager.notification.renameFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.renameFailed"));
    }
  }

  /* 打开重命名对话框 */
  function openRenameModal(file: FileInfo) {
    renameTarget = file;
    newName = file.name;
    showRenameModal = true;
  }

  /* 删除选中的文件 */
  async function deleteSelected() {
    if (selectedPaths.size === 0) return;

    const deleteCount = selectedPaths.size;
    try {
      const response: any = await fileService.delete(Array.from(selectedPaths));
      if (response.success === 200 || response.success === true) {
        showDeleteConfirm = false;
        selectedPaths = new Set();
        refresh();
        showNotification("success", $t("fileManager.notification.deleted", { values: { n: deleteCount } }));
      } else {
        showNotification("error", response.message || $t("fileManager.notification.deleteFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.deleteFailed"));
    }
  }

  /* 复制到剪贴板 */
  function copyToClipboard() {
    if (selectedPaths.size === 0) return;
    clipboard = { paths: Array.from(selectedPaths), operation: "copy" };
    showNotification("info", $t("fileManager.notification.copied", { values: { n: selectedPaths.size } }));
  }

  /* 剪切到剪贴板 */
  function cutToClipboard() {
    if (selectedPaths.size === 0) return;
    clipboard = { paths: Array.from(selectedPaths), operation: "cut" };
    showNotification("info", $t("fileManager.notification.cut", { values: { n: selectedPaths.size } }));
  }

  /* 粘贴 */
  async function paste() {
    if (!clipboard) return;

    try {
      const response: any = await fileService.copyMove(
        clipboard.paths,
        currentPath,
        clipboard.operation === "cut",
      );

      if (response.success === 200 || response.success === true) {
        if (clipboard.operation === "cut") {
          clipboard = null;
        }
        refresh();
        showNotification("success", $t("fileManager.notification.pasteSuccess"));
      } else {
        showNotification("error", response.message || $t("fileManager.notification.pasteFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.pasteFailed"));
    }
  }

  /* 从此处打开终端 */
  async function openTerminalHere(path?: string) {
    const targetPath = path || currentPath;
    await windowManager.open("terminal", { initialPath: targetPath });
  }

  /* 移动到 - 打开对话框 */
  async function openMoveToModal() {
    if (selectedPaths.size === 0) return;
    moveToPath = currentPath;
    moveToHistory = [currentPath];
    showMoveToModal = true;
    await loadMoveToFiles(currentPath);
  }

  /* 移动到 - 加载目录 */
  async function loadMoveToFiles(path: string) {
    moveToLoading = true;
    try {
      const response = await fileService.list(path, showHidden);
      if (response.success && response.data) {
        // 只显示文件夹
        moveToFiles = response.data.content.filter((f) => f.is_dir);
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.loadDirFailed"));
    } finally {
      moveToLoading = false;
    }
  }

  /* 移动到 - 导航到目录 */
  async function navigateMoveTo(path: string) {
    moveToPath = path;
    moveToHistory = [...moveToHistory, path];
    await loadMoveToFiles(path);
  }

  /* 移动到 - 返回上级 */
  async function moveToGoUp() {
    if (moveToPath === "/") return;
    const parent = moveToPath.split("/").slice(0, -1).join("/") || "/";
    moveToPath = parent;
    moveToHistory = [...moveToHistory, parent];
    await loadMoveToFiles(parent);
  }

  /* 移动到 - 确认移动 */
  async function confirmMoveTo() {
    if (selectedPaths.size === 0) return;
    
    // 不能移动到自身或子目录
    const pathsToMove = Array.from(selectedPaths);
    for (const p of pathsToMove) {
      if (moveToPath === p || moveToPath.startsWith(p + "/")) {
        showNotification("error", $t("fileManager.notification.cannotMoveToSelf"));
        return;
      }
    }

    try {
      const response: any = await fileService.copyMove(pathsToMove, moveToPath, true);
      if (response.success === 200 || response.success === true) {
        showNotification("success", $t("fileManager.notification.moved", { values: { n: pathsToMove.length } }));
        showMoveToModal = false;
        selectedPaths = new Set();
        refresh();
      } else {
        showNotification("error", response.message || $t("fileManager.notification.moveFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.moveFailed"));
    }
  }

  /* 生成唯一ID */
  function generateId(): string {
    return Date.now().toString(36) + Math.random().toString(36).substr(2);
  }

  /* 上传文件 - 点击按钮触发 */
  function triggerUpload() {
    const input = document.createElement("input");
    input.type = "file";
    input.multiple = true;
    input.onchange = (e) => {
      const target = e.target as HTMLInputElement;
      if (target.files?.length) {
        uploadFiles(Array.from(target.files));
      }
    };
    input.click();
  }

  /* 上传文件夹 - 点击按钮触发 */
  function triggerFolderUpload() {
    const input = document.createElement("input");
    input.type = "file";
    input.webkitdirectory = true;
    input.onchange = (e) => {
      const target = e.target as HTMLInputElement;
      if (target.files?.length) {
        // webkitRelativePath 包含完整的相对路径，如 "folder/sub/file.txt"
        const filesWithPath = Array.from(target.files).map(file => ({
          file,
          relativePath: (file as any).webkitRelativePath as string || file.name
        }));
        uploadFilesWithPath(filesWithPath);
      }
    };
    input.click();
  }

  /* 处理 input change 事件 */
  async function handleUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.length) return;

    const fileList = Array.from(input.files);
    await uploadFiles(fileList);
    input.value = "";
  }

  /* 上传文件列表 - 使用分片上传（无相对路径） */
  async function uploadFiles(fileList: File[]) {
    const filesWithPath = fileList.map(file => ({ file, relativePath: file.name }));
    await uploadFilesWithPath(filesWithPath);
  }

  /* 上传文件列表 - 支持相对路径（文件夹结构） */
  async function uploadFilesWithPath(filesWithPath: { file: File; relativePath: string }[]) {
    if (filesWithPath.length === 0) return;

    showUploadPanel = true;

    // 最大文件大小 10GB
    const MAX_SIZE = 10 * 1024 * 1024 * 1024;

    // 检查文件大小
    for (const { file } of filesWithPath) {
      if (file.size > MAX_SIZE) {
        showNotification("error", $t("fileManager.notification.fileTooLarge", { values: { name: file.name } }));
        return;
      }
    }

    // 添加到上传队列
    const newItems: UploadItem[] = filesWithPath.map(({ file, relativePath }) => ({
      id: generateId(),
      file,
      relativePath,
      progress: 0,
      status: "pending" as const,
      speed: 0,
      remainingTime: 0,
    }));

    uploadQueue = [...uploadQueue, ...newItems];

    // 并发上传（最多5个并行）
    const MAX_CONCURRENT = 5;
    const chunks: UploadItem[][] = [];
    for (let i = 0; i < newItems.length; i += MAX_CONCURRENT) {
      chunks.push(newItems.slice(i, i + MAX_CONCURRENT));
    }

    for (const chunk of chunks) {
      await Promise.all(chunk.map(item => startUpload(item.id)));
    }

    refresh();
  }

  /* 开始单个文件上传 */
  async function startUpload(itemId: string) {
    const idx = uploadQueue.findIndex((i) => i.id === itemId);
    if (idx === -1) return;

    const item = uploadQueue[idx];
    if (item.status === "uploading") return;

    // 创建上传器
    const uploader = new ChunkedUploader({
      path: currentPath,
      file: item.file,
      subPath: item.relativePath, // 传递相对路径用于文件夹结构
      chunkSize: 50 * 1024 * 1024, // 50MB
      maxRetries: 3,
      timeout: 10 * 60 * 1000, // 10分钟
      onProgress: (progress: UploadProgress) => {
        const i = uploadQueue.findIndex((u) => u.id === itemId);
        if (i !== -1) {
          uploadQueue[i].progress = progress.percent;
          uploadQueue[i].speed = progress.speed;
          uploadQueue[i].remainingTime = progress.remainingTime;
          uploadQueue = [...uploadQueue]; // 触发响应式更新
        }
      },
      onStateChange: (state: UploadState) => {
        const i = uploadQueue.findIndex((u) => u.id === itemId);
        if (i !== -1) {
          if (state === "completed") {
            uploadQueue[i].status = "done";
            uploadQueue[i].progress = 100;
          } else if (state === "error") {
            uploadQueue[i].status = "error";
          } else if (state === "paused") {
            uploadQueue[i].status = "paused";
          } else if (state === "uploading") {
            uploadQueue[i].status = "uploading";
          }
          uploadQueue = [...uploadQueue];
        }
      },
      onError: (error: Error) => {
        const i = uploadQueue.findIndex((u) => u.id === itemId);
        if (i !== -1) {
          uploadQueue[i].error = error.message;
          uploadQueue = [...uploadQueue];
        }
      },
    });

    // 保存uploader引用用于暂停/恢复
    uploadQueue[idx].uploader = uploader;
    uploadQueue[idx].status = "uploading";
    uploadQueue = [...uploadQueue];

    // 开始上传
    const result = await uploader.start();

    if (!result.success && uploadQueue[idx]?.status !== "paused") {
      const i = uploadQueue.findIndex((u) => u.id === itemId);
      if (i !== -1) {
        uploadQueue[i].status = "error";
        uploadQueue[i].error = result.message;
        uploadQueue = [...uploadQueue];
      }
    }
  }

  /* 暂停上传 */
  function pauseUpload(itemId: string) {
    const idx = uploadQueue.findIndex((i) => i.id === itemId);
    if (idx !== -1 && uploadQueue[idx].uploader) {
      uploadQueue[idx].uploader!.pause();
      uploadQueue[idx].status = "paused";
      uploadQueue = [...uploadQueue];
    }
  }

  /* 恢复上传 */
  async function resumeUpload(itemId: string) {
    const idx = uploadQueue.findIndex((i) => i.id === itemId);
    if (idx !== -1 && uploadQueue[idx].uploader) {
      await uploadQueue[idx].uploader!.resume();
    } else {
      // 如果没有uploader实例，重新开始上传
      await startUpload(itemId);
    }
  }

  /* 取消上传 */
  async function cancelUpload(itemId: string) {
    const idx = uploadQueue.findIndex((i) => i.id === itemId);
    if (idx !== -1) {
      if (uploadQueue[idx].uploader) {
        await uploadQueue[idx].uploader!.cancel();
      }
      uploadQueue = uploadQueue.filter((i) => i.id !== itemId);
    }
  }

  /* 重试上传 */
  async function retryUpload(itemId: string) {
    const idx = uploadQueue.findIndex((i) => i.id === itemId);
    if (idx !== -1) {
      uploadQueue[idx].status = "pending";
      uploadQueue[idx].progress = 0;
      uploadQueue[idx].error = undefined;
      uploadQueue[idx].uploader = undefined;
      uploadQueue = [...uploadQueue];
      await startUpload(itemId);
    }
  }

  /* 清除已完成的上传 */
  function clearCompletedUploads() {
    uploadQueue = uploadQueue.filter((item) => item.status !== "done" && item.status !== "error");
    if (uploadQueue.length === 0) {
      showUploadPanel = false;
    }
  }

  /* 拖拽上传 - 改进版 */
  function handleDragEnter(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    dragCounter++;
    if (e.dataTransfer?.types.includes("Files")) {
      isDragOver = true;
    }
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer) {
      e.dataTransfer.dropEffect = "copy";
    }
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    dragCounter--;
    if (dragCounter === 0) {
      isDragOver = false;
    }
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragOver = false;
    dragCounter = 0;

    if (!e.dataTransfer?.items.length) return;

    // 使用 webkitGetAsEntry 递归读取文件夹
    const items = Array.from(e.dataTransfer.items);
    const entries = items
      .map(item => item.webkitGetAsEntry?.())
      .filter((entry): entry is FileSystemEntry => entry !== null);

    if (entries.length > 0) {
      readEntriesRecursively(entries);
    } else if (e.dataTransfer?.files.length) {
      // 回退到普通文件上传
      uploadFiles(Array.from(e.dataTransfer.files));
    }
  }

  /* 递归读取文件系统条目 */
  async function readEntriesRecursively(entries: FileSystemEntry[]) {
    const filesWithPath: { file: File; relativePath: string }[] = [];

    async function traverse(entry: FileSystemEntry, path: string) {
      if (entry.isFile) {
        const fileEntry = entry as FileSystemFileEntry;
        const file = await new Promise<File>((resolve, reject) => {
          fileEntry.file(resolve, reject);
        });
        filesWithPath.push({ file, relativePath: path });
      } else if (entry.isDirectory) {
        const dirEntry = entry as FileSystemDirectoryEntry;
        const reader = dirEntry.createReader();
        
        // 读取所有条目（可能需要多次调用）
        let allEntries: FileSystemEntry[] = [];
        let batch: FileSystemEntry[];
        do {
          batch = await new Promise<FileSystemEntry[]>((resolve, reject) => {
            reader.readEntries(resolve, reject);
          });
          allEntries = allEntries.concat(batch);
        } while (batch.length > 0);

        for (const childEntry of allEntries) {
          await traverse(childEntry, path + '/' + childEntry.name);
        }
      }
    }

    try {
      for (const entry of entries) {
        await traverse(entry, entry.name);
      }
      
      if (filesWithPath.length > 0) {
        uploadFilesWithPath(filesWithPath);
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.readFolderFailed", { values: { error: String(err) } }));
    }
  }

  /* 内部文件拖拽 - 开始 */
  const INTERNAL_DRAG_TYPE = "application/x-rde-files";
  
  function handleFileDragStart(e: DragEvent, file: FileInfo) {
    // 如果拖拽的文件在选中集合中，移动所有选中文件；否则只移动单个文件
    if (selectedPaths.has(file.path)) {
      internalDragPaths = Array.from(selectedPaths);
    } else {
      internalDragPaths = [file.path];
      selectedPaths = new Set([file.path]);
    }
    
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData(INTERNAL_DRAG_TYPE, JSON.stringify(internalDragPaths));
      // 设置拖拽图标
      const dragIcon = document.createElement("div");
      const tr = get(translate);
      dragIcon.textContent = tr("fileManager.dragDrop.nItems", { values: { n: internalDragPaths.length } });
      dragIcon.style.cssText = "position:absolute;top:-1000px;padding:8px 12px;background:#333;color:#fff;border-radius:4px;font-size:12px;";
      document.body.appendChild(dragIcon);
      e.dataTransfer.setDragImage(dragIcon, 0, 0);
      setTimeout(() => dragIcon.remove(), 0);
    }
  }

  function handleFileDragEnd() {
    internalDragPaths = [];
    dropTargetFolder = null;
  }

  /* 内部文件拖拽 - 拖拽经过文件夹 */
  function handleFolderDragOver(e: DragEvent, folder: FileInfo) {
    if (!folder.is_dir) return;
    // 检查是否为内部拖拽
    if (e.dataTransfer?.types.includes(INTERNAL_DRAG_TYPE)) {
      e.preventDefault();
      e.stopPropagation();
      e.dataTransfer.dropEffect = "move";
      dropTargetFolder = folder.path;
    }
  }

  function handleFolderDragLeave(e: DragEvent, folder: FileInfo) {
    if (dropTargetFolder === folder.path) {
      dropTargetFolder = null;
    }
  }

  /* 内部文件拖拽 - 放置到文件夹 */
  async function handleFolderDrop(e: DragEvent, folder: FileInfo) {
    if (!folder.is_dir) return;
    
    // 检查是否为内部拖拽
    const data = e.dataTransfer?.getData(INTERNAL_DRAG_TYPE);
    if (!data) return;
    
    e.preventDefault();
    e.stopPropagation();
    dropTargetFolder = null;
    
    try {
      const paths: string[] = JSON.parse(data);
      
      // 不能移动到自身或子目录
      for (const p of paths) {
        if (folder.path === p || folder.path.startsWith(p + "/")) {
          showNotification("error", $t("fileManager.notification.cannotMoveToSelf"));
          internalDragPaths = [];
          return;
        }
      }
      
      const response: any = await fileService.copyMove(paths, folder.path, true);
      if (response.success === 200 || response.success === true) {
        showNotification("success", $t("fileManager.notification.movedTo", { values: { n: paths.length, folder: folder.name } }));
        selectedPaths = new Set();
        refresh();
      } else {
        showNotification("error", response.message || $t("fileManager.notification.moveFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.moveFailed"));
    } finally {
      internalDragPaths = [];
    }
  }

  /* 侧边栏拖拽放置 */
  function handleSidebarDragOver(e: DragEvent, path: string) {
    if (e.dataTransfer?.types.includes(INTERNAL_DRAG_TYPE)) {
      e.preventDefault();
      e.stopPropagation();
      e.dataTransfer.dropEffect = "move";
      dropTargetFolder = path;
    }
  }

  function handleSidebarDragLeave(e: DragEvent, path: string) {
    if (dropTargetFolder === path) {
      dropTargetFolder = null;
    }
  }

  async function handleSidebarDrop(e: DragEvent, targetPath: string) {
    const data = e.dataTransfer?.getData(INTERNAL_DRAG_TYPE);
    if (!data) return;
    
    e.preventDefault();
    e.stopPropagation();
    dropTargetFolder = null;
    
    try {
      const paths: string[] = JSON.parse(data);
      
      // 不能移动到自身或子目录
      for (const p of paths) {
        if (targetPath === p || targetPath.startsWith(p + "/")) {
          showNotification("error", $t("fileManager.notification.cannotMoveToSelf"));
          internalDragPaths = [];
          return;
        }
      }
      
      const response: any = await fileService.copyMove(paths, targetPath, true);
      if (response.success === 200 || response.success === true) {
        showNotification("success", $t("fileManager.notification.moved", { values: { n: paths.length } }));
        selectedPaths = new Set();
        refresh();
      } else {
        showNotification("error", response.message || $t("fileManager.notification.moveFailed"));
      }
    } catch (err) {
      showNotification("error", $t("fileManager.notification.moveFailed"));
    } finally {
      internalDragPaths = [];
    }
  }

  /* 右键菜单 */
  let contextMenu = $state<{ x: number; y: number; file?: FileInfo } | null>(null);

  function handleContextMenu(e: MouseEvent, file?: FileInfo) {
    e.preventDefault();
    e.stopPropagation(); // 阻止事件冒泡到桌面
    if (file && !selectedPaths.has(file.path)) {
      selectedPaths = new Set([file.path]);
    }
    contextMenu = { x: e.clientX, y: e.clientY, file };
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  /* 键盘快捷键 */
  function handleKeydown(e: KeyboardEvent) {
    // 如果有模态框打开，或者焦点在输入框中，不处理快捷键
    const target = e.target as HTMLElement;
    if (
      showNewFolderModal ||
      showRenameModal ||
      showMoveToModal ||
      showDeleteConfirm ||
      showApkInstallModal ||
      showElevateModal ||
      target.tagName === "INPUT" ||
      target.tagName === "TEXTAREA" ||
      target.isContentEditable
    ) {
      return;
    }

    if (e.ctrlKey || e.metaKey) {
      switch (e.key) {
        case "a":
          e.preventDefault();
          selectAll();
          break;
        case "c":
          e.preventDefault();
          copyToClipboard();
          break;
        case "x":
          e.preventDefault();
          cutToClipboard();
          break;
        case "v":
          e.preventDefault();
          paste();
          break;
        case "r":
          e.preventDefault();
          refresh();
          break;
      }
    } else {
      switch (e.key) {
        case "Delete":
          if (selectedPaths.size > 0) {
            showDeleteConfirm = true;
          }
          break;
        case "F2":
          if (selectedPaths.size === 1) {
            const file = files.find((f) => selectedPaths.has(f.path));
            if (file) openRenameModal(file);
          }
          break;
        case "Escape":
          deselectAll();
          closeContextMenu();
          break;
        case "Backspace":
          goUp();
          break;
      }
    }
  }

  /* 获取文件扩展名（从文件名中提取） */
  function getFileExtension(file: FileInfo): string {
    if (file.is_dir) return "";
    // 优先使用 extension 字段（如果后端提供），否则从文件名提取
    if (file.extension) return file.extension.toLowerCase();
    const name = file.name;
    const dotIndex = name.lastIndexOf(".");
    if (dotIndex === -1 || dotIndex === 0) return "";
    return name.substring(dotIndex + 1).toLowerCase();
  }

  /* 判断是否为图片 */
  function isImage(file: FileInfo): boolean {
    const ext = getFileExtension(file);
    return ["jpg", "jpeg", "png", "gif", "webp", "svg", "bmp"].includes(ext);
  }

  /* 判断是否为视频 */
  function isVideo(file: FileInfo): boolean {
    const ext = getFileExtension(file);
    return ["mp4", "mkv", "avi", "mov", "wmv", "webm", "flv", "m4v", "3gp"].includes(ext);
  }

  /* 判断是否支持缩略图 */
  function supportsThumbnail(file: FileInfo): boolean {
    return isImage(file) || isVideo(file);
  }

  /* 获取缩略图 URL */
  function getThumbnailUrl(file: FileInfo): string {
    const size = viewMode === "grid" ? 256 : 128;
    return fileService.getThumbnailUrl(file.path, size);
  }

  /* 获取文件预览 URL */
  function getPreviewUrl(file: FileInfo): string {
    return fileService.getPreviewUrl(file.path);
  }

  /* 初始化 */
  /* 管理员提权 */
  async function doElevate() {
    if (!elevatePassword) return;
    elevateError = "";
    elevateLoading = true;

    try {
      const resp: any = await fileService.elevate(elevatePassword);
      if (resp.success === 200 && resp.data?.elevated) {
        elevated = true;
        elevateExpiresAt = new Date(resp.data.expires_at);
        startElevateCountdown();
        showElevateModal = false;
        elevatePassword = "";
        needElevate = false;
        showNotification("success", $t("fileManager.notification.elevateSuccess"));
        // 自动重试之前因权限被拒绝的操作
        loadFiles();
      } else {
        elevateError = resp.message || $t("fileManager.notification.elevateFailed");
      }
    } catch (err) {
      elevateError = err instanceof Error ? err.message : $t("fileManager.notification.elevateFailed");
    } finally {
      elevateLoading = false;
    }
  }

  async function revokeElevation() {
    try {
      await fileService.revokeElevation();
    } catch {}
    elevated = false;
    elevateExpiresAt = null;
    elevateRemaining = 0;
    if (elevateTimer) {
      clearInterval(elevateTimer);
      elevateTimer = null;
    }
    showNotification("info", $t("fileManager.notification.elevateRevoked"));
    loadFiles();
  }

  function startElevateCountdown() {
    if (elevateTimer) clearInterval(elevateTimer);
    elevateTimer = setInterval(() => {
      if (elevateExpiresAt) {
        const remaining = Math.max(0, Math.floor((elevateExpiresAt.getTime() - Date.now()) / 1000));
        elevateRemaining = remaining;
        if (remaining <= 0) {
          elevated = false;
          elevateExpiresAt = null;
          if (elevateTimer) {
            clearInterval(elevateTimer);
            elevateTimer = null;
          }
          showNotification("info", $t("fileManager.notification.elevateExpired"));
          loadFiles();
        }
      }
    }, 1000);
  }

  function formatElevateTime(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = seconds % 60;
    return `${m}:${s.toString().padStart(2, "0")}`;
  }

  async function checkElevationStatus() {
    try {
      const resp: any = await fileService.getElevationStatus();
      if (resp.success === 200 && resp.data?.elevated) {
        elevated = true;
        elevateExpiresAt = resp.data.expires_at ? new Date(resp.data.expires_at) : null;
        elevateRemaining = Math.floor(resp.data.remaining || 0);
        startElevateCountdown();
      }
    } catch {}
  }

  onMount(async () => {
    // 获取用户书签（含 home 目录路径）
    try {
      const bookmarks = await fileService.getBookmarks();
      quickAccessDefaultRaw = bookmarks.default || quickAccessDefaultRaw;
      quickAccessSystem = bookmarks.system || [];
      // 优先使用传入的 initialPath，否则使用用户 home 目录
      if (initialPath) {
        currentPath = initialPath;
      } else if (bookmarks.home_path && bookmarks.home_path !== "/") {
        currentPath = bookmarks.home_path;
      }
    } catch (e) {
      console.warn("Failed to load bookmarks", e);
      // 即使获取书签失败，也使用 initialPath
      if (initialPath) {
        currentPath = initialPath;
      }
    }

    // 检查是否已有提权会话
    if (userStore.isAdmin) {
      await checkElevationStatus();
    }

    history = [currentPath];
    historyIndex = 0;
    loadFiles();

    windowManager.setTitle(windowId, $t("fileManager.titleWithPath", { values: { path: currentPath } }));
  });

  onDestroy(() => {
    if (elevateTimer) {
      clearInterval(elevateTimer);
      elevateTimer = null;
    }
  });
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_noninteractive_element_interactions, a11y_click_events_have_key_events -->
<div
  class="file-manager"
  onclick={closeContextMenu}
  oncontextmenu={(e) => handleContextMenu(e)}
  ondragenter={handleDragEnter}
  ondragover={handleDragOver}
  ondragleave={handleDragLeave}
  ondrop={handleDrop}
  role="application"
>
  <!-- 侧边栏 -->
  <aside class="sidebar" class:collapsed={sidebarCollapsed}>
    <div class="sidebar-header">
      <span class="sidebar-title">{$t("fileManager.sidebar.quickAccess")}</span>
      <button class="collapse-btn" onclick={() => (sidebarCollapsed = !sidebarCollapsed)}>
        <Icon icon={sidebarCollapsed ? "mdi:chevron-right" : "mdi:chevron-left"} />
      </button>
    </div>

    {#if !sidebarCollapsed}
      <nav class="quick-access">
        {#each quickAccessDefault as item}
          <button
            class="nav-item"
            class:active={currentPath === item.path}
            class:drop-target={dropTargetFolder === item.path}
            onclick={() => navigateTo(item.path)}
            ondragover={(e) => handleSidebarDragOver(e, item.path)}
            ondragleave={(e) => handleSidebarDragLeave(e, item.path)}
            ondrop={(e) => handleSidebarDrop(e, item.path)}
          >
            <Icon icon={item.icon} width="18" />
            <span>{item.label}</span>
          </button>
        {/each}

        {#if quickAccessSystem.length > 0}
          <div class="sidebar-divider"></div>
          <span class="sidebar-section-label">{$t("fileManager.sidebar.system")}</span>
          {#each quickAccessSystem as item}
            <button
              class="nav-item"
              class:active={currentPath === item.path}
              class:drop-target={dropTargetFolder === item.path}
              onclick={() => navigateTo(item.path)}
              ondragover={(e) => handleSidebarDragOver(e, item.path)}
              ondragleave={(e) => handleSidebarDragLeave(e, item.path)}
              ondrop={(e) => handleSidebarDrop(e, item.path)}
            >
              <Icon icon={item.icon} width="18" />
              <span>{item.label}</span>
            </button>
          {/each}
        {/if}
      </nav>

      {#if clipboard}
        <div class="clipboard-info">
          <Icon icon={clipboard.operation === "copy" ? "mdi:content-copy" : "mdi:content-cut"} />
          <span
            >{clipboard.operation === "copy"
              ? $t("fileManager.sidebar.itemsCopied", { values: { n: clipboard.paths.length } })
              : $t("fileManager.sidebar.itemsCut", { values: { n: clipboard.paths.length } })}</span
          >
        </div>
      {/if}
    {/if}
  </aside>

  <main class="main-content">
    <!-- 工具栏 -->
    <header class="toolbar">
      <div class="nav-buttons">
        <button onclick={goBack} title={$t("fileManager.toolbar.back") + " (Alt+←)"} disabled={loading || historyIndex <= 0}>
          <Icon icon="mdi:arrow-left" />
        </button>
        <button
          onclick={goForward}
          title={$t("fileManager.toolbar.forward") + " (Alt+→)"}
          disabled={loading || historyIndex >= history.length - 1}
        >
          <Icon icon="mdi:arrow-right" />
        </button>
        <button onclick={goUp} title={$t("fileManager.toolbar.parentDir")} disabled={loading || currentPath === "/"}>
          <Icon icon="mdi:arrow-up" />
        </button>
        <button onclick={refresh} title={$t("fileManager.toolbar.refresh") + " (Ctrl+R)"} disabled={loading} class:loading>
          <Icon icon="mdi:refresh" />
        </button>
      </div>

      {#if elevated}
        <button class="elevate-badge" onclick={revokeElevation} title={$t("fileManager.elevate.clickToExit")}>
          <Icon icon="mdi:shield-check" width="16" />
          <span>{$t("fileManager.elevate.admin")} {formatElevateTime(elevateRemaining)}</span>
        </button>
      {/if}

      <!-- 地址栏（面包屑/编辑模式切换） -->
      <div class="address-bar" class:editing={addressBarEditing}>
        {#if addressBarEditing}
          <!-- 编辑模式 -->
          <input
            type="text"
            class="address-input"
            bind:value={addressBarInput}
            bind:this={addressBarInputRef}
            onkeydown={(e) => {
              if (e.key === "Enter") {
                const path = addressBarInput.trim() || "/";
                addressBarEditing = false;
                navigateTo(path.startsWith("/") ? path : "/" + path);
              } else if (e.key === "Escape") {
                addressBarEditing = false;
              }
            }}
            onblur={() => {
              // 延迟关闭，允许点击按钮
              setTimeout(() => { addressBarEditing = false; }, 150);
            }}
            placeholder={$t("fileManager.toolbar.inputPath")}
          />
          <button
            class="address-btn confirm"
            title={$t("fileManager.toolbar.confirm") + " (Enter)"}
            onmousedown={(e) => e.preventDefault()}
            onclick={() => {
              const path = addressBarInput.trim() || "/";
              addressBarEditing = false;
              navigateTo(path.startsWith("/") ? path : "/" + path);
            }}
          >
            <Icon icon="mdi:check" />
          </button>
          <button
            class="address-btn cancel"
            title={$t("fileManager.toolbar.cancel") + " (Esc)"}
            onmousedown={(e) => e.preventDefault()}
            onclick={() => { addressBarEditing = false; }}
          >
            <Icon icon="mdi:close" />
          </button>
        {:else}
          <!-- 面包屑模式 -->
          <nav
            class="breadcrumb"
            onclick={(e) => {
              // 点击空白区域进入编辑模式
              if (e.target === e.currentTarget) {
                addressBarInput = currentPath;
                addressBarEditing = true;
                setTimeout(() => addressBarInputRef?.select(), 0);
              }
            }}
            role="navigation"
          >
            {#each breadcrumbs as crumb, i}
              {#if i > 0}
                <Icon icon="mdi:chevron-right" class="separator" />
              {/if}
              <button
                class="crumb"
                class:current={i === breadcrumbs.length - 1}
                onclick={() => navigateTo(crumb.path)}
              >
                {crumb.label}
              </button>
            {/each}
          </nav>
          <button
            class="address-btn edit"
            title={$t("fileManager.toolbar.editPath")}
            onclick={() => {
              addressBarInput = currentPath;
              addressBarEditing = true;
              setTimeout(() => addressBarInputRef?.select(), 0);
            }}
          >
            <Icon icon="mdi:pencil" />
          </button>
        {/if}
      </div>

      <div class="actions">
        <button onclick={triggerUpload} title={$t("fileManager.toolbar.uploadFile")}>
          <Icon icon="mdi:upload" />
        </button>
        <button onclick={triggerFolderUpload} title={$t("fileManager.toolbar.uploadFolder")}>
          <Icon icon="mdi:folder-upload" />
        </button>
        <button
          onclick={() => {
            newFolderName = "";
            showNewFolderModal = true;
          }}
          title={$t("fileManager.toolbar.newFolder")}
        >
          <Icon icon="mdi:folder-plus" />
        </button>
        <button onclick={copyToClipboard} title={$t("fileManager.toolbar.copy") + " (Ctrl+C)"} disabled={selectedPaths.size === 0}>
          <Icon icon="mdi:content-copy" />
        </button>
        <button onclick={cutToClipboard} title={$t("fileManager.toolbar.cut") + " (Ctrl+X)"} disabled={selectedPaths.size === 0}>
          <Icon icon="mdi:content-cut" />
        </button>
        <button onclick={paste} title={$t("fileManager.toolbar.paste") + " (Ctrl+V)"} disabled={!clipboard}>
          <Icon icon="mdi:content-paste" />
        </button>
        <button
          class="delete"
          onclick={() => (showDeleteConfirm = true)}
          title={$t("fileManager.toolbar.delete") + " (Delete)"}
          disabled={selectedPaths.size === 0}
        >
          <Icon icon="mdi:delete" />
        </button>
      </div>

      <div class="view-options">
        <button
          class:active={viewMode === "grid"}
          onclick={() => (viewMode = "grid")}
          title={$t("fileManager.toolbar.gridView")}
        >
          <Icon icon="mdi:view-grid" />
        </button>
        <button
          class:active={viewMode === "list"}
          onclick={() => (viewMode = "list")}
          title={$t("fileManager.toolbar.listView")}
        >
          <Icon icon="mdi:view-list" />
        </button>
        <button
          class:active={viewMode === "detail"}
          onclick={() => (viewMode = "detail")}
          title={$t("fileManager.toolbar.detailView")}
        >
          <Icon icon="mdi:view-headline" />
        </button>

        <select bind:value={sortBy} class="sort-select">
          <option value="name">{$t("fileManager.toolbar.sortByName")}</option>
          <option value="size">{$t("fileManager.toolbar.sortBySize")}</option>
          <option value="modified">{$t("fileManager.toolbar.sortByModified")}</option>
        </select>
        <button onclick={() => (sortAsc = !sortAsc)} title={sortAsc ? $t("fileManager.toolbar.ascending") : $t("fileManager.toolbar.descending")}>
          <Icon icon={sortAsc ? "mdi:sort-ascending" : "mdi:sort-descending"} />
        </button>
      </div>

      <div class="search-bar">
        <Icon icon="mdi:magnify" />
        <input type="text" placeholder={$t("fileManager.toolbar.searchFiles")} bind:value={searchKeyword} />
        {#if searchKeyword}
          <button class="clear-search" onclick={() => (searchKeyword = "")}>
            <Icon icon="mdi:close" />
          </button>
        {/if}
      </div>
    </header>

    <!-- 文件列表 -->
      <div class="file-list-container">
        {#if isDragOver}
          <div class="drag-overlay">
            <Icon icon="mdi:cloud-upload" width="64" />
            <span class="drag-title">{$t("fileManager.dragDrop.releaseToUpload")}</span>
            <span class="drag-subtitle">{$t("fileManager.dragDrop.supportLargeFile")}</span>
          </div>
        {/if}

        {#if loading}
          <div class="loading-overlay">
            <Icon icon="mdi:loading" class="spin" width="32" />
            <span>{$t("fileManager.status.loading")}</span>
          </div>
        {:else if needElevate}
          <div class="permission-denied-message">
            <Icon icon="mdi:lock" width="48" />
            <span class="permission-title">{$t("fileManager.permission.denied")}</span>
            <span class="permission-desc">{$t("fileManager.permission.noAccessDir")}</span>
            {#if userStore.isAdmin}
              <Button variant="primary" onclick={() => { elevatePassword = ""; elevateError = ""; showElevateModal = true; }}>
                <Icon icon="mdi:shield-key" />
                {$t("fileManager.permission.accessAsAdmin")}
              </Button>
            {/if}
            <Button variant="outline" onclick={goUp}>
              <Icon icon="mdi:arrow-up" />
              {$t("fileManager.permission.goBack")}
            </Button>
          </div>
        {:else if error}
          <div class="error-message">
            <Icon icon="mdi:alert-circle" width="32" />
            <span>{error}</span>
            <Button onclick={refresh}>{$t("fileManager.error.retry")}</Button>
          </div>
        {:else if sortedFiles.length === 0}
          <div class="empty-message">
            <Icon icon="mdi:folder-open" width="64" />
            <span>{searchKeyword ? $t("fileManager.empty.noMatches") : $t("fileManager.empty.noFiles")}</span>
            {#if !searchKeyword}
              <div class="empty-actions">
                <Button
                  variant="outline"
                  onclick={() => {
                    newFolderName = "";
                    showNewFolderModal = true;
                  }}
                >
                  <Icon icon="mdi:folder-plus" />
                  {$t("fileManager.toolbar.newFolder")}
                </Button>
                <Button variant="primary" onclick={triggerUpload}>
                  <Icon icon="mdi:upload" />
                  {$t("fileManager.toolbar.uploadFile")}
                </Button>
              </div>
            {/if}
          </div>
        {:else}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="file-list"
            class:grid={viewMode === "grid"}
            class:list={viewMode === "list"}
            class:detail={viewMode === "detail"}
            oncontextmenu={(e) => handleContextMenu(e)}
          >
            {#if viewMode === "detail"}
              <div class="detail-header">
                <span class="col-icon"></span>
                <span class="col-name">{$t("fileManager.detailHeader.name")}</span>
                <span class="col-size">{$t("fileManager.detailHeader.size")}</span>
                <span class="col-modified">{$t("fileManager.detailHeader.modified")}</span>
                <span class="col-owner">{$t("fileManager.detailHeader.owner")}</span>
                <span class="col-group">{$t("fileManager.detailHeader.group")}</span>
                <span class="col-mode">{$t("fileManager.detailHeader.permissions")}</span>
              </div>
            {/if}
            {#each sortedFiles as file (file.path)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="file-item"
                class:selected={selectedPaths.has(file.path)}
                class:cut={clipboard?.operation === "cut" && clipboard.paths.includes(file.path)}
                class:drop-target={dropTargetFolder === file.path}
                class:dragging={internalDragPaths.includes(file.path)}
                draggable="true"
                ondragstart={(e) => handleFileDragStart(e, file)}
                ondragend={handleFileDragEnd}
                ondragover={(e) => handleFolderDragOver(e, file)}
                ondragleave={(e) => handleFolderDragLeave(e, file)}
                ondrop={(e) => handleFolderDrop(e, file)}
                onclick={(e) => handleItemClick(file, e)}
                ondblclick={() => handleItemDoubleClick(file)}
                oncontextmenu={(e) => handleContextMenu(e, file)}
              >
                <div class="file-icon" style="color: {getFileColor(file)}">
                  {#if viewMode === "grid" && supportsThumbnail(file)}
                    <LazyThumbnail
                      src={getThumbnailUrl(file)}
                      alt={file.name}
                      fallbackIcon={getFileIcon(file)}
                      isVideo={isVideo(file)}
                      size={80}
                    />
                  {:else}
                    <Icon icon={getFileIcon(file)} width={viewMode === "grid" ? 48 : 20} />
                  {/if}
                  {#if file.is_symlink}
                    <span class="symlink-badge" title={$t("fileManager.status.symlinkTo", { values: { target: file.link_target || '' } })}>
                      <Icon icon="mdi:arrow-right-bold-circle" width={viewMode === "grid" ? 16 : 10} />
                    </span>
                  {/if}
                </div>
                <div class="file-info">
                  <span class="file-name" title={file.name}>{file.name}</span>
                  {#if viewMode === "list" || viewMode === "detail"}
                    <span class="file-size">{file.is_dir ? "-" : formatSize(file.size)}</span>
                    <span class="file-date">{formatDate(file.modified)}</span>
                  {/if}
                  {#if viewMode === "detail"}
                    <span class="file-owner">{file.owner || "-"}</span>
                    <span class="file-group">{file.group || "-"}</span>
                    <span class="file-mode">{file.mode || "-"}</span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

    <!-- 状态栏 -->
    <footer class="statusbar">
      <span class="item-count">{$t("fileManager.status.itemCount", { values: { n: sortedFiles.length } })}</span>
      {#if selectedPaths.size > 0}
        <span class="selected-info">
          {$t("fileManager.status.selectedCount", { values: { n: selectedPaths.size } })}
          {#if selectedSize > 0}
            ({formatSize(selectedSize)})
          {/if}
        </span>
      {/if}
      <span class="spacer"></span>
      <div class="show-hidden">
        <Checkbox id="show-hidden" bind:checked={showHidden} onchange={refresh} />
        <label for="show-hidden">{$t("fileManager.status.showHiddenFiles")}</label>
      </div>
    </footer>
  </main>

  <!-- 右键菜单 -->
  {#if contextMenu}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="context-menu" style="left: {contextMenu.x}px; top: {contextMenu.y}px">
      {#if contextMenu.file}
        <button
          onclick={() => {
            handleItemDoubleClick(contextMenu!.file!);
            closeContextMenu();
          }}
        >
          <Icon icon={contextMenu.file.is_dir ? "mdi:folder-open" : "mdi:eye"} />
          <span>{contextMenu.file.is_dir ? $t("fileManager.contextMenu.open") : $t("fileManager.contextMenu.preview")}</span>
        </button>
        {#if contextMenu.file.is_dir}
          <button
            onclick={() => {
              openTerminalHere(contextMenu!.file!.path);
              closeContextMenu();
            }}
          >
            <Icon icon="mdi:console" />
            <span>{$t("fileManager.contextMenu.openTerminalHere")}</span>
          </button>
        {/if}
        {#if !contextMenu.file.is_dir}
          <button
            onclick={() => {
              fileService.download(contextMenu!.file!.path, contextMenu!.file!.name);
              closeContextMenu();
            }}
          >
            <Icon icon="mdi:download" />
            <span>{$t("fileManager.contextMenu.download")}</span>
          </button>
        {/if}
        <hr />
        <button
          onclick={() => {
            copyToClipboard();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:content-copy" />
          <span>{$t("fileManager.contextMenu.copy")}</span>
          <kbd>Ctrl+C</kbd>
        </button>
        <button
          onclick={() => {
            cutToClipboard();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:content-cut" />
          <span>{$t("fileManager.contextMenu.cut")}</span>
          <kbd>Ctrl+X</kbd>
        </button>
        <button
          onclick={() => {
            openMoveToModal();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:folder-move" />
          <span>{$t("fileManager.contextMenu.moveTo")}</span>
        </button>
        <button
          onclick={() => {
            openRenameModal(contextMenu!.file!);
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:rename-box" />
          <span>{$t("fileManager.contextMenu.rename")}</span>
          <kbd>F2</kbd>
        </button>
        <hr />
        <button
          class="danger"
          onclick={() => {
            showDeleteConfirm = true;
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:delete" />
          <span>{$t("fileManager.contextMenu.delete")}</span>
          <kbd>Del</kbd>
        </button>
      {:else}
        {#if clipboard}
          <button
            onclick={() => {
              paste();
              closeContextMenu();
            }}
          >
            <Icon icon="mdi:content-paste" />
            <span>{$t("fileManager.contextMenu.paste")}</span>
            <kbd>Ctrl+V</kbd>
          </button>
          <hr />
        {/if}
        <button
          onclick={() => {
            newFolderName = "";
            showNewFolderModal = true;
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:folder-plus" />
          <span>{$t("fileManager.contextMenu.newFolder")}</span>
        </button>
        <button
          onclick={() => {
            triggerUpload();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:upload" />
          <span>{$t("fileManager.contextMenu.uploadFile")}</span>
        </button>
        <button
          onclick={() => {
            openTerminalHere();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:console" />
          <span>{$t("fileManager.contextMenu.openTerminalHere")}</span>
        </button>
        <hr />
        <button
          onclick={() => {
            selectAll();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:select-all" />
          <span>{$t("fileManager.contextMenu.selectAll")}</span>
          <kbd>Ctrl+A</kbd>
        </button>
        <button
          onclick={() => {
            refresh();
            closeContextMenu();
          }}
        >
          <Icon icon="mdi:refresh" />
          <span>{$t("fileManager.contextMenu.refresh")}</span>
          <kbd>Ctrl+R</kbd>
        </button>
      {/if}
    </div>
  {/if}

  <!-- 上传面板 -->
  {#if showUploadPanel}
    <div class="upload-panel">
      <div class="upload-panel-header">
        <span>{$t("fileManager.upload.queue")} ({uploadQueue.length})</span>
        <button onclick={clearCompletedUploads}>{$t("fileManager.upload.clearCompleted")}</button>
        <button onclick={() => (showUploadPanel = false)}>
          <Icon icon="mdi:close" />
        </button>
      </div>
      <div class="upload-list">
        {#each uploadQueue as item (item.id)}
          <div
            class="upload-item"
            class:done={item.status === "done"}
            class:error={item.status === "error"}
            class:paused={item.status === "paused"}
          >
            <div class="upload-item-info">
              <Icon
                icon={item.status === "done"
                  ? "mdi:check-circle"
                  : item.status === "error"
                    ? "mdi:alert-circle"
                    : item.status === "paused"
                      ? "mdi:pause-circle"
                      : "mdi:file"}
              />
              <div class="upload-item-details">
                <span class="upload-name" title={item.relativePath || item.file.name}>
                  {item.relativePath || item.file.name}
                </span>
                <div class="upload-meta">
                  <span class="upload-size">{formatSize(item.file.size)}</span>
                  {#if item.status === "uploading" && item.speed > 0}
                    <span class="upload-speed">{formatSize(item.speed)}/s</span>
                    <span class="upload-remaining">{$t("fileManager.upload.remaining", { values: { time: formatTime(item.remainingTime) } })}</span>
                  {/if}
                  {#if item.error}
                    <span class="upload-error">{item.error}</span>
                  {/if}
                </div>
              </div>
            </div>

            {#if item.status === "uploading" || item.status === "paused"}
              <Progress value={item.progress} max={100} size="sm" />
            {/if}

            <div class="upload-item-actions">
              {#if item.status === "uploading"}
                <button class="action-btn-sm" onclick={() => pauseUpload(item.id)} title={$t("fileManager.upload.pause")}>
                  <Icon icon="mdi:pause" width="16" />
                </button>
              {:else if item.status === "paused"}
                <button class="action-btn-sm" onclick={() => resumeUpload(item.id)} title={$t("fileManager.upload.resume")}>
                  <Icon icon="mdi:play" width="16" />
                </button>
              {:else if item.status === "error"}
                <button class="action-btn-sm" onclick={() => retryUpload(item.id)} title={$t("fileManager.upload.retry")}>
                  <Icon icon="mdi:refresh" width="16" />
                </button>
              {/if}
              {#if item.status !== "done"}
                <button
                  class="action-btn-sm danger"
                  onclick={() => cancelUpload(item.id)}
                  title={$t("fileManager.upload.cancel")}
                >
                  <Icon icon="mdi:close" width="16" />
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- 通知消息 -->
  {#if notification}
    <div class="notification {notification.type}">
      <Icon
        icon={notification.type === "success"
          ? "mdi:check-circle"
          : notification.type === "error"
            ? "mdi:alert-circle"
            : "mdi:information"}
      />
      <span>{notification.message}</span>
    </div>
  {/if}
</div>

<!-- 新建文件夹弹窗 -->
<Modal bind:open={showNewFolderModal} title={$t("fileManager.modal.newFolder")} showFooter>
  <div class="modal-form">
    <Input label={$t("fileManager.modal.folderName")} bind:value={newFolderName} placeholder={$t("fileManager.modal.enterFolderName")} />
  </div>
  {#snippet footer()}
    <Button variant="outline" onclick={() => (showNewFolderModal = false)}>{$t("common.cancel")}</Button>
    <Button variant="primary" onclick={createFolder} disabled={!newFolderName.trim()}>{$t("fileManager.modal.create")}</Button>
  {/snippet}
</Modal>

<!-- 重命名弹窗 -->
<Modal bind:open={showRenameModal} title={$t("fileManager.modal.rename")} showFooter>
  <div class="modal-form">
    <Input label={$t("fileManager.modal.newName")} bind:value={newName} placeholder={$t("fileManager.modal.enterNewName")} />
  </div>
  {#snippet footer()}
    <Button variant="outline" onclick={() => (showRenameModal = false)}>{$t("common.cancel")}</Button>
    <Button variant="primary" onclick={renameFile} disabled={!newName.trim()}>{$t("common.confirm")}</Button>
  {/snippet}
</Modal>

<!-- 删除确认弹窗 -->
<Modal bind:open={showDeleteConfirm} title={$t("fileManager.modal.confirmDelete")} showFooter>
  <Alert variant="warning">
    {$t("fileManager.modal.deleteWarning", { values: { n: selectedPaths.size } })}
  </Alert>
  {#snippet footer()}
    <Button variant="outline" onclick={() => (showDeleteConfirm = false)}>{$t("common.cancel")}</Button>
    <Button variant="destructive" onclick={deleteSelected}>{$t("common.delete")}</Button>
  {/snippet}
</Modal>

<!-- 移动到对话框 -->
<Modal bind:open={showMoveToModal} title={$t("fileManager.modal.moveTo")} showFooter>
  <div class="move-to-modal">
    <div class="move-to-path-bar">
      <button class="move-to-up-btn" onclick={moveToGoUp} disabled={moveToPath === "/"}>
        <Icon icon="mdi:arrow-up" />
      </button>
      <span class="move-to-current-path">{moveToPath}</span>
    </div>
    <div class="move-to-list">
      {#if moveToLoading}
        <div class="move-to-loading">
          <Icon icon="mdi:loading" class="spin" />
          {$t("fileManager.status.loading")}
        </div>
      {:else if moveToFiles.length === 0}
        <div class="move-to-empty">{$t("fileManager.modal.noFoldersHere")}</div>
      {:else}
        {#each moveToFiles as folder (folder.path)}
          <button
            class="move-to-item"
            ondblclick={() => navigateMoveTo(folder.path)}
            onclick={() => navigateMoveTo(folder.path)}
          >
            <Icon icon="mdi:folder" />
            <span>{folder.name}</span>
          </button>
        {/each}
      {/if}
    </div>
    <div class="move-to-selected">
      {$t("fileManager.modal.willMove", { values: { n: selectedPaths.size } })} <strong>{moveToPath}</strong>
    </div>
  </div>
  {#snippet footer()}
    <Button variant="outline" onclick={() => (showMoveToModal = false)}>{$t("common.cancel")}</Button>
    <Button variant="primary" onclick={confirmMoveTo} disabled={moveToLoading}>{$t("fileManager.modal.moveToHere")}</Button>
  {/snippet}
</Modal>

<!-- 文件预览弹窗 -->
<Modal bind:open={showPreviewModal} title={previewFile?.name || $t("fileManager.preview.title")} size="xl" showFooter>
  {#if previewFile}
    {@const category = getFileCategory(previewFile)}
    <div class="preview-content" class:text-editor={category === "text"}>
      {#if category === "image"}
        <img src={getPreviewUrl(previewFile)} alt={previewFile.name} class="preview-image" />
      {:else if category === "video"}
        <video src={getPreviewUrl(previewFile)} controls autoplay class="preview-video">
          <track kind="captions" />
          {$t("fileManager.preview.videoNotSupported")}
        </video>
      {:else if category === "audio"}
        <div class="preview-audio-container">
          <Icon icon="mdi:music-circle" width="120" />
          <p class="audio-name">{previewFile.name}</p>
          <audio src={getPreviewUrl(previewFile)} controls autoplay class="preview-audio">
            {$t("fileManager.preview.audioNotSupported")}
          </audio>
        </div>
      {:else if category === "pdf"}
        <iframe src={getPreviewUrl(previewFile)} class="preview-pdf" title={previewFile.name}
        ></iframe>
      {:else if category === "text"}
        {#if fileContentLoading}
          <div class="text-loading">
            <Icon icon="mdi:loading" class="spin" width="32" />
            <span>{$t("fileManager.status.loading")}</span>
          </div>
        {:else}
          <textarea
            class="text-editor-area"
            bind:value={fileContent}
            oninput={() => (fileContentEdited = true)}
            spellcheck="false"
          ></textarea>
        {/if}
      {:else}
        <div class="preview-info">
          <Icon icon="mdi:file" width="64" />
          <p>{$t("fileManager.preview.file")}: {previewFile.name}</p>
          <p>{$t("fileManager.preview.size")}: {formatSize(previewFile.size)}</p>
          <p>{$t("fileManager.preview.modified")}: {formatDate(previewFile.modified)}</p>
        </div>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    {#if previewFile}
      {@const category = getFileCategory(previewFile)}
      {#if category === "text" && fileContentEdited}
        <Button variant="primary" onclick={saveFileContent}>
          <Icon icon="mdi:content-save" />
          {$t("fileManager.preview.save")}
        </Button>
      {/if}
      <Button variant="outline" onclick={() => (showPreviewModal = false)}>{$t("fileManager.preview.close")}</Button>
      <Button
        variant="outline"
        onclick={() => {
          fileService.download(previewFile!.path, previewFile!.name);
        }}
      >
        <Icon icon="mdi:download" />
        {$t("fileManager.preview.download")}
      </Button>
    {/if}
  {/snippet}
</Modal>

<!-- 管理员提权弹窗 -->
<Modal bind:open={showElevateModal} title={$t("fileManager.elevate.title")} showFooter>
  <div class="elevate-content">
    <div class="elevate-icon">
      <Icon icon="mdi:shield-key" width="48" />
    </div>
    <p class="elevate-desc">{$t("fileManager.elevate.description")}</p>
    <Input
      type="password"
      placeholder={$t("fileManager.elevate.placeholder")}
      bind:value={elevatePassword}
      onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') doElevate(); }}
    />
    {#if elevateError}
      <Alert variant="error">{elevateError}</Alert>
    {/if}
  </div>
  {#snippet footer()}
    <Button variant="outline" onclick={() => { showElevateModal = false; elevatePassword = ''; }}>{$t("common.cancel")}</Button>
    <Button variant="primary" onclick={doElevate} disabled={elevateLoading || !elevatePassword}>
      {#if elevateLoading}
        <Icon icon="mdi:loading" class="spin" />
      {/if}
      {$t("common.confirm")}
    </Button>
  {/snippet}
</Modal>

<style>
  .file-manager {
    display: flex;
    height: 100%;
    background: var(--bg-window, #fafafa);
    color: var(--text-primary, #333);
    position: relative;
    overflow: hidden;
  }

  /* 侧边栏 */
  .sidebar {
    width: 200px;
    background: var(--bg-sidebar, #f5f5f5);
    border-right: 1px solid var(--border-color, #e0e0e0);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    transition: width 0.2s ease;

    &.collapsed {
      width: 40px;

      .sidebar-title,
      .quick-access,
      .clipboard-info {
        display: none;
      }
    }
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
  }

  .sidebar-title {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-secondary, #666);
    text-transform: uppercase;
  }

  .collapse-btn {
    width: 24px;
    height: 24px;
    border: none;
    background: transparent;
    border-radius: 4px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary, #666);

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.05));
    }
  }

  .quick-access {
    padding: 8px;
    display: flex;
    flex-direction: column;
    gap: 2px;

    .nav-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 8px 12px;
      border: none;
      background: transparent;
      border-radius: 6px;
      cursor: pointer;
      font-size: 13px;
      color: var(--text-primary, #333);
      transition: all 0.15s;
      text-align: left;

      &:hover {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
      }

      &.active {
        background: var(--color-primary, #0066cc);
        color: white;
      }

      &.drop-target {
        background: var(--bg-active, rgba(0, 102, 204, 0.2));
        outline: 2px dashed var(--color-primary, #0066cc);
        outline-offset: -2px;
      }
    }
  }

  .sidebar-divider {
    height: 1px;
    background: var(--border-color, rgba(0, 0, 0, 0.1));
    margin: 6px 12px;
  }

  .sidebar-section-label {
    font-size: 11px;
    color: var(--text-secondary, #999);
    padding: 2px 12px 4px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .clipboard-info {
    margin: 8px;
    padding: 8px 12px;
    background: var(--bg-active, rgba(0, 102, 204, 0.1));
    border-radius: 6px;
    font-size: 12px;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--color-primary, #0066cc);
  }

  /* 主内容区 */
  .main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-window-header, white);
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    flex-shrink: 0;
    flex-wrap: wrap;
  }

  .nav-buttons,
  .actions,
  .view-options {
    display: flex;
    gap: 2px;

    button {
      width: 32px;
      height: 32px;
      border: none;
      border-radius: 6px;
      background: transparent;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--text-secondary, #555);
      transition: all 0.15s;

      &:hover:not(:disabled) {
        background: var(--bg-hover, #f0f0f0);
        color: var(--text-primary, #333);
      }

      &:disabled {
        opacity: 0.4;
        cursor: not-allowed;
      }

      &.active {
        background: var(--bg-active, #e0e0e0);
        color: var(--color-primary, #0066cc);
      }

      &.loading :global(svg) {
        animation: spin 1s linear infinite;
      }

      &.delete:hover:not(:disabled) {
        color: #f44336;
      }
    }
  }

  .sort-select {
    height: 32px;
    padding: 0 8px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    background: var(--bg-sidebar, #f5f5f5);
    font-size: 12px;
    color: var(--text-primary, #333);
    cursor: pointer;
    outline: none;
  }

  /* 地址栏容器 */
  .address-bar {
    flex: 1;
    min-width: 100px;
    display: flex;
    align-items: center;
    gap: 4px;
    height: 32px;
    background: var(--bg-sidebar, #f5f5f5);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    overflow: hidden;
    transition: border-color 0.15s;

    &.editing {
      border-color: var(--primary-color, #3b82f6);
      background: var(--bg-primary, #fff);
    }
  }

  .address-input {
    flex: 1;
    border: none;
    background: none;
    padding: 0 8px;
    height: 100%;
    font-size: 13px;
    font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
    color: var(--text-primary, #333);
    outline: none;

    &::placeholder {
      color: var(--text-muted, #aaa);
    }
  }

  .address-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    background: none;
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-secondary, #666);
    flex-shrink: 0;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.05));
      color: var(--text-primary, #333);
    }

    &.confirm:hover {
      color: var(--success-color, #22c55e);
    }

    &.cancel:hover {
      color: var(--error-color, #ef4444);
    }

    &.edit {
      margin-right: 2px;
    }
  }

  /* 面包屑 */
  .breadcrumb {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 8px;
    height: 100%;
    overflow-x: auto;
    cursor: text;

    /* 隐藏滚动条但保持可滚动 */
    scrollbar-width: none;
    -ms-overflow-style: none;
    &::-webkit-scrollbar {
      display: none;
    }

    :global(.separator) {
      color: var(--text-muted, #aaa);
      flex-shrink: 0;
    }

    .crumb {
      border: none;
      background: none;
      padding: 4px 8px;
      border-radius: 4px;
      font-size: 13px;
      color: var(--text-secondary, #666);
      cursor: pointer;
      white-space: nowrap;
      flex-shrink: 0;

      &:hover {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        color: var(--text-primary, #333);
      }

      &.current {
        color: var(--text-primary, #333);
        font-weight: 500;
      }
    }
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 10px;
    height: 32px;
    background: var(--bg-sidebar, #f5f5f5);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    color: var(--text-secondary, #888);

    input {
      width: 140px;
      border: none;
      background: transparent;
      font-size: 13px;
      outline: none;
      color: var(--text-primary, #333);

      &::placeholder {
        color: var(--text-muted, #aaa);
      }
    }

    .clear-search {
      width: 20px;
      height: 20px;
      border: none;
      background: transparent;
      border-radius: 50%;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--text-muted, #aaa);

      &:hover {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        color: var(--text-primary, #333);
      }
    }
  }

  .file-list-container {
    flex: 1;
    overflow: auto;
    position: relative;
  }

  .drag-overlay {
    position: absolute;
    inset: 0;
    background: rgba(0, 102, 204, 0.1);
    border: 3px dashed var(--color-primary, #0066cc);
    border-radius: 8px;
    margin: 12px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--color-primary, #0066cc);
    z-index: 10;
    font-size: 18px;
    font-weight: 500;
  }

  .loading-overlay,
  .error-message,
  .empty-message {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    color: var(--text-secondary, #666);

    :global(.spin) {
      animation: spin 1s linear infinite;
    }
  }

  .empty-actions {
    display: flex;
    gap: 12px;
    margin-top: 8px;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  .file-list {
    padding: 12px;

    &.grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(110px, 1fr));
      gap: 8px;

      .file-item {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 12px 8px;
        border-radius: 8px;
        cursor: pointer;
        transition: all 0.15s;
        border: 2px solid transparent;

        &:hover {
          background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        }

        &.selected {
          background: var(--bg-active, rgba(0, 102, 204, 0.1));
          border-color: var(--color-primary, #0066cc);
        }

        &.cut {
          opacity: 0.5;
        }

        &.dragging {
          opacity: 0.4;
        }

        &.drop-target {
          background: var(--bg-active, rgba(0, 102, 204, 0.15));
          border-color: var(--color-primary, #0066cc);
          border-style: dashed;
        }

        .file-icon {
          margin-bottom: 8px;
          width: 80px;
          height: 80px;
          display: flex;
          align-items: center;
          justify-content: center;
          position: relative;
        }

        .file-info {
          text-align: center;
          width: 100%;

          .file-name {
            display: block;
            font-size: 12px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            max-width: 100px;
          }
        }
      }
    }

    &.list {
      .file-item {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 12px;
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.15s;
        border: 2px solid transparent;

        &:hover {
          background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        }

        &.selected {
          background: var(--bg-active, rgba(0, 102, 204, 0.1));
          border-color: var(--color-primary, #0066cc);
        }

        &.cut {
          opacity: 0.5;
        }

        &.dragging {
          opacity: 0.4;
        }

        &.drop-target {
          background: var(--bg-active, rgba(0, 102, 204, 0.15));
          border-color: var(--color-primary, #0066cc);
          border-style: dashed;
        }

        .file-icon {
          flex-shrink: 0;
          position: relative;
        }

        .file-info {
          flex: 1;
          display: flex;
          align-items: center;
          gap: 16px;
          min-width: 0;

          .file-name {
            flex: 1;
            font-size: 13px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
          }

          .file-size {
            width: 80px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            text-align: right;
            flex-shrink: 0;
          }

          .file-date {
            width: 150px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            flex-shrink: 0;
          }
        }
      }
    }

    &.detail {
      .detail-header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 8px 12px;
        background: var(--bg-subtle, rgba(0, 0, 0, 0.03));
        border-bottom: 1px solid var(--border-color, #e0e0e0);
        font-size: 12px;
        font-weight: 600;
        color: var(--text-secondary, #666);

        .col-icon {
          width: 20px;
          flex-shrink: 0;
        }

        .col-name {
          flex: 1;
          min-width: 150px;
        }

        .col-size {
          width: 80px;
          text-align: right;
        }

        .col-modified {
          width: 150px;
        }

        .col-owner {
          width: 80px;
        }

        .col-group {
          width: 80px;
        }

        .col-mode {
          width: 90px;
          font-family: monospace;
        }
      }

      .file-item {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 6px 12px;
        border-radius: 0;
        cursor: pointer;
        transition: all 0.15s;
        border-left: 2px solid transparent;
        border-right: 2px solid transparent;

        &:hover {
          background: var(--bg-hover, rgba(0, 0, 0, 0.03));
        }

        &.selected {
          background: var(--bg-active, rgba(0, 102, 204, 0.1));
          border-left-color: var(--color-primary, #0066cc);
        }

        &.cut {
          opacity: 0.5;
        }

        &.dragging {
          opacity: 0.4;
        }

        &.drop-target {
          background: var(--bg-active, rgba(0, 102, 204, 0.15));
          border-left-color: var(--color-primary, #0066cc);
          border-style: dashed;
        }

        .file-icon {
          width: 20px;
          flex-shrink: 0;
          position: relative;
          display: flex;
          align-items: center;
          justify-content: center;
        }

        .file-info {
          flex: 1;
          display: flex;
          align-items: center;
          gap: 12px;
          min-width: 0;

          .file-name {
            flex: 1;
            min-width: 150px;
            font-size: 13px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
          }

          .file-size {
            width: 80px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            text-align: right;
            flex-shrink: 0;
          }

          .file-date {
            width: 150px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            flex-shrink: 0;
          }

          .file-owner {
            width: 80px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            flex-shrink: 0;
          }

          .file-group {
            width: 80px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            flex-shrink: 0;
          }

          .file-mode {
            width: 90px;
            font-size: 12px;
            color: var(--text-secondary, #888);
            font-family: monospace;
            flex-shrink: 0;
          }
        }
      }
    }
  }

  .statusbar {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 8px 12px;
    background: var(--bg-window-header, white);
    border-top: 1px solid var(--border-color, #e0e0e0);
    font-size: 12px;
    color: var(--text-secondary, #666);

    .spacer {
      flex: 1;
    }

    .show-hidden {
      display: flex;
      align-items: center;
      gap: 6px;
      cursor: pointer;

      * {
        cursor: pointer;
      }
    }
  }

  .context-menu {
    position: fixed;
    min-width: 180px;
    background: var(--bg-menu, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    padding: 4px 0;
    z-index: 1000;

    button {
      display: flex;
      align-items: center;
      gap: 10px;
      width: 100%;
      padding: 8px 12px;
      border: none;
      background: none;
      text-align: left;
      font-size: 13px;
      cursor: pointer;
      color: var(--text-primary, #333);

      &:hover {
        background: var(--bg-hover, #f0f0f0);
      }

      &.danger {
        color: #f44336;
      }

      kbd {
        margin-left: auto;
        font-size: 11px;
        color: var(--text-muted, #aaa);
        font-family: inherit;
      }
    }

    hr {
      margin: 4px 0;
      border: none;
      border-top: 1px solid var(--border-color, #e0e0e0);
    }
  }

  /* 上传面板 */
  .upload-panel {
    position: absolute;
    bottom: 50px;
    right: 16px;
    width: 320px;
    background: var(--bg-menu, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    z-index: 100;
  }

  .upload-panel-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    font-size: 13px;
    font-weight: 500;

    button {
      border: none;
      background: none;
      font-size: 12px;
      color: var(--color-primary, #0066cc);
      cursor: pointer;

      &:last-child {
        margin-left: auto;
        color: var(--text-muted, #aaa);
      }
    }
  }

  .upload-list {
    max-height: 300px;
    overflow: auto;
  }

  .upload-item {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px 12px;
    font-size: 12px;
    border-bottom: 1px solid var(--border-color, #f0f0f0);

    &:last-child {
      border-bottom: none;
    }

    &.done {
      .upload-item-info :global(svg) {
        color: #4caf50;
      }
    }

    &.error {
      .upload-item-info :global(svg) {
        color: #f44336;
      }
    }

    &.paused {
      .upload-item-info :global(svg) {
        color: #ff9800;
      }
    }
  }

  .upload-item-info {
    display: flex;
    align-items: flex-start;
    gap: 10px;

    :global(svg) {
      flex-shrink: 0;
      margin-top: 2px;
    }
  }

  .upload-item-details {
    flex: 1;
    min-width: 0;
  }

  .upload-name {
    display: block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: 500;
    color: var(--text-primary, #333);
  }

  .upload-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 4px;
    font-size: 11px;
    color: var(--text-muted, #999);
  }

  .upload-size {
    color: var(--text-muted, #999);
  }

  .upload-speed {
    color: var(--color-primary, #0066cc);
  }

  .upload-remaining {
    color: var(--text-muted, #999);
  }

  .upload-error {
    color: #f44336;
  }

  .upload-item-actions {
    display: flex;
    justify-content: flex-end;
    gap: 4px;
    margin-top: 4px;
  }

  .action-btn-sm {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    border: none;
    border-radius: 4px;
    background: var(--bg-hover, #f0f0f0);
    cursor: pointer;
    color: var(--text-secondary, #666);
    transition: all 0.15s ease;

    &:hover {
      background: var(--bg-active, #e0e0e0);
    }

    &.danger:hover {
      background: #ffebee;
      color: #f44336;
    }
  }

  /* 拖拽提示优化 */
  .drag-overlay {
    position: absolute;
    inset: 0;
    background: rgba(255, 255, 255, 0.95);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    z-index: 50;
    border: 3px dashed var(--color-primary, #0066cc);
    border-radius: 8px;
    margin: 8px;

    :global(svg) {
      color: var(--color-primary, #0066cc);
    }

    .drag-title {
      font-size: 18px;
      font-weight: 600;
      color: var(--text-primary, #333);
    }

    .drag-subtitle {
      font-size: 13px;
      color: var(--text-muted, #999);
    }
  }

  /* 通知 */
  .notification {
    position: absolute;
    bottom: 60px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    background: var(--bg-menu, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    font-size: 13px;
    animation: slideUp 0.2s ease;
    z-index: 200;

    &.success {
      color: #4caf50;
      border-color: #4caf50;
    }

    &.error {
      color: #f44336;
      border-color: #f44336;
    }

    &.info {
      color: var(--color-primary, #0066cc);
      border-color: var(--color-primary, #0066cc);
    }
  }

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateX(-50%) translateY(10px);
    }
    to {
      opacity: 1;
      transform: translateX(-50%) translateY(0);
    }
  }

  /* 弹窗内容 */
  .modal-form {
    padding: 8px 0;
  }

  .preview-content {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 300px;

    &.text-editor {
      min-height: 400px;
      align-items: stretch;
    }

    .preview-image {
      max-width: 100%;
      max-height: 70vh;
      border-radius: 8px;
      object-fit: contain;
    }

    .preview-video {
      max-width: 100%;
      max-height: 70vh;
      border-radius: 8px;
      background: #000;
    }

    .preview-audio-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 16px;
      padding: 40px;

      :global(svg) {
        color: var(--color-primary, #0066cc);
      }

      .audio-name {
        font-size: 18px;
        font-weight: 500;
        color: var(--text-primary, #333);
      }

      .preview-audio {
        width: 100%;
        max-width: 400px;
      }
    }

    .preview-pdf {
      width: 100%;
      height: 70vh;
      border: none;
      border-radius: 8px;
    }

    .text-loading {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 12px;
      color: var(--text-muted, #999);
    }

    .text-editor-area {
      width: 100%;
      min-height: 400px;
      padding: 16px;
      border: 1px solid var(--border-color, #e0e0e0);
      border-radius: 8px;
      font-family:
        "JetBrains Mono", "Fira Code", "SF Mono", Menlo, Monaco, "Courier New", monospace;
      font-size: 13px;
      line-height: 1.6;
      resize: vertical;
      background: var(--bg-input, #fff);
      color: var(--text-primary, #333);

      &:focus {
        outline: none;
        border-color: var(--color-primary, #0066cc);
        box-shadow: 0 0 0 3px rgba(0, 102, 204, 0.1);
      }
    }

    .preview-info {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 8px;
      text-align: center;
      color: var(--text-secondary, #666);

      :global(svg) {
        color: var(--text-muted, #ccc);
        margin-bottom: 8px;
      }

      p {
        margin: 4px 0;
      }
    }
  }

  /* APK 安装弹窗 */
  .apk-install-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 20px;
    gap: 12px;

    :global(.android-icon) {
      color: #3ddc84;
    }

    .apk-icon {
      width: 64px;
      height: 64px;
      border-radius: 12px;
      object-fit: contain;
    }

    .apk-name {
      font-size: 16px;
      font-weight: 500;
      color: var(--text-primary, #333);
      word-break: break-all;
      text-align: center;
    }

    .apk-package {
      font-size: 12px;
      color: var(--text-tertiary, #999);
      font-family: monospace;
    }

    .apk-meta {
      display: flex;
      gap: 16px;
      flex-wrap: wrap;
      justify-content: center;

      .meta-item {
        display: flex;
        align-items: center;
        gap: 4px;
        font-size: 13px;
        color: var(--text-secondary, #666);
      }
    }

    .apk-size {
      font-size: 14px;
      color: var(--text-secondary, #666);
    }
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }

  /* 管理员提权徽章 */
  .elevate-badge {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 10px;
    background: linear-gradient(135deg, #f59e0b, #d97706);
    color: white;
    border: none;
    border-radius: 12px;
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
    animation: pulse-glow 2s ease-in-out infinite;
    transition: all 0.2s;
  }

  .elevate-badge:hover {
    background: linear-gradient(135deg, #d97706, #b45309);
    transform: scale(1.05);
  }

  @keyframes pulse-glow {
    0%, 100% { box-shadow: 0 0 4px rgba(245, 158, 11, 0.3); }
    50% { box-shadow: 0 0 8px rgba(245, 158, 11, 0.6); }
  }

  /* 权限不足页面 */
  .permission-denied-message {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 48px 24px;
    color: var(--text-secondary, #999);
    text-align: center;
  }

  .permission-denied-message :global(svg) {
    color: var(--danger, #ef4444);
    opacity: 0.7;
  }

  .permission-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .permission-desc {
    font-size: 13px;
    color: var(--text-secondary, #999);
    max-width: 300px;
  }

  /* 符号链接小箭头标记 */
  .symlink-badge {
    position: absolute;
    bottom: 0;
    right: 0;
    color: var(--text-tertiary, #888);
    line-height: 1;
    display: flex;
    filter: drop-shadow(0 0 1px var(--bg-primary, #fff));
  }

  /* 提权弹窗 */
  .elevate-content {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 8px 0;
  }

  .elevate-icon {
    color: var(--warning, #f59e0b);
  }

  .elevate-desc {
    font-size: 13px;
    color: var(--text-secondary, #999);
    text-align: center;
    margin: 0;
    line-height: 1.5;
  }

  /* 移动到对话框 */
  .move-to-modal {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 400px;
  }

  .move-to-path-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 6px;
  }

  .move-to-up-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border: none;
    background: var(--bg-tertiary, #eee);
    border-radius: 4px;
    cursor: pointer;
    color: var(--text-primary, #333);
    &:hover:not(:disabled) {
      background: var(--bg-hover, #ddd);
    }
    &:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
  }

  .move-to-current-path {
    flex: 1;
    font-size: 13px;
    color: var(--text-primary, #333);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .move-to-list {
    max-height: 300px;
    overflow-y: auto;
    border: 1px solid var(--border-color, #e5e5e5);
    border-radius: 6px;
    background: var(--bg-primary, #fff);
  }

  .move-to-loading,
  .move-to-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px 20px;
    color: var(--text-secondary, #999);
    font-size: 13px;
  }

  .move-to-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 14px;
    border: none;
    background: transparent;
    cursor: pointer;
    text-align: left;
    font-size: 13px;
    color: var(--text-primary, #333);
    transition: background 0.15s;
    &:hover {
      background: var(--bg-secondary, #f5f5f5);
    }
    &:not(:last-child) {
      border-bottom: 1px solid var(--border-light, #eee);
    }
    :global(svg) {
      color: var(--folder-color, #f7b500);
      flex-shrink: 0;
    }
    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .move-to-selected {
    padding: 8px 12px;
    font-size: 12px;
    color: var(--text-secondary, #666);
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 6px;
    strong {
      color: var(--text-primary, #333);
    }
  }
</style>
