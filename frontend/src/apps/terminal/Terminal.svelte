<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { terminalService } from "$shared/services/terminal";
  import { sshService, type SSHConnection } from "$shared/services/ssh";
  import { theme as systemTheme } from "$shared/stores/theme.svelte";
  import Icon from "@iconify/svelte";
  import SFTPBrowser from "./SFTPBrowser.svelte";
  import { t } from "svelte-i18n";

  let { windowId, initialPath }: { windowId: string; initialPath?: string } = $props();

  // 连接类型
  type ConnectionType = 'local' | 'ssh';

  // 终端实例类型
  interface TerminalPane {
    id: string;
    terminal: any;
    fitAddon: any;
    searchAddon: any;
    sessionId: string;
    connected: boolean;
    connecting: boolean;
    ws: WebSocket | null;
    element: HTMLDivElement | null;
    reconnectAttempts: number;
    reconnectTimer: number | null;
    shouldReconnect: boolean;
    // SSH 相关
    connectionType: ConnectionType;
    sshConnectionId?: string;
    sshConnectionName?: string;
  }

  // 分屏节点类型
  type SplitDirection = 'horizontal' | 'vertical';
  
  interface SplitNode {
    id: string;
    type: 'pane' | 'split';
    // pane 类型
    paneId?: string;
    // split 类型
    direction?: SplitDirection;
    first?: SplitNode;
    second?: SplitNode;
    ratio?: number; // 第一个区域的比例 (0-1)
  }

  // Tab 类型定义（支持分屏）
  interface TerminalTab {
    id: string;
    title: string;
    layout: SplitNode; // 分屏布局根节点
    activePaneId: string; // 当前活动的 pane ID
    connectionType: ConnectionType;
    sshConnectionId?: string;
  }

  // 自动重连配置
  const RECONNECT_MAX_ATTEMPTS = 5;
  const RECONNECT_BASE_DELAY = 1000;

  // 所有终端实例
  let panes = $state<TerminalPane[]>([]);
  let paneCounter = 0;

  // 多标签页状态
  let tabs = $state<TerminalTab[]>([]);
  let activeTabId = $state<string>("");
  let tabCounter = 0;

  // 派生当前活动标签页和活动 pane
  let activeTab = $derived(tabs.find(t => t.id === activeTabId));
  let activePane = $derived(activeTab ? panes.find(p => p.id === activeTab.activePaneId) : undefined);

  // 全局状态
  let fontSize = $state(14);

  // 搜索状态
  let showSearch = $state(false);
  let searchQuery = $state("");
  let searchInputRef: HTMLInputElement | null = $state(null);

  // 右键菜单状态
  let showContextMenu = $state(false);
  let contextMenuPos = $state({ x: 0, y: 0 });

  // 分屏拖拽状态
  let draggingSplit = $state<string | null>(null);

  // SSH 连接状态
  let sshConnections = $state<SSHConnection[]>([]);
  let showSSHPanel = $state(false);
  let showSSHModal = $state(false);
  let editingConnection = $state<SSHConnection | null>(null);
  let sshForm = $state({
    name: "",
    host: "",
    port: 22,
    username: "",
    password: "",
    auth_type: "password" as "password" | "key",
    private_key: "",
    passphrase: "",
    group: "",
    description: "",
  });
  let sshTesting = $state(false);
  let sshSaving = $state(false);
  let sshError = $state("");
  let sshShaking = $state(false);

  // SFTP 浏览器状态
  let showSFTPBrowser = $state(false);
  let sftpSessionId = $state("");
  let sftpConnectionName = $state("");

  // 快捷命令
  interface QuickCommand {
    id: string;
    name: string;
    command: string;
    icon?: string;
  }

  // 获取默认快捷命令（使用 i18n）
  function getDefaultQuickCommands(): QuickCommand[] {
    return [
      { id: "1", name: $t("terminal.listFiles"), command: "ls -la", icon: "mdi:folder-outline" },
      { id: "2", name: $t("terminal.viewProcesses"), command: "htop || top", icon: "mdi:chart-line" },
      { id: "3", name: $t("terminal.diskSpace"), command: "df -h", icon: "mdi:harddisk" },
      { id: "4", name: $t("terminal.memoryUsage"), command: "free -h", icon: "mdi:memory" },
      { id: "5", name: "Docker", command: "docker ps -a", icon: "mdi:docker" },
    ];
  }

  let quickCommands = $state<QuickCommand[]>([]);
  let showQuickCommands = $state(false);
  let editingCommand = $state<QuickCommand | null>(null);
  let newCommandName = $state("");
  let newCommandText = $state("");

  // 初始化快捷命令
  function initQuickCommands() {
    const saved = localStorage.getItem("terminal-quick-commands");
    if (saved) {
      try {
        quickCommands = JSON.parse(saved);
      } catch {
        quickCommands = getDefaultQuickCommands();
      }
    } else {
      quickCommands = getDefaultQuickCommands();
    }
  }

  // 保存快捷命令
  function saveQuickCommands() {
    localStorage.setItem("terminal-quick-commands", JSON.stringify(quickCommands));
  }

  // 执行快捷命令
  function executeQuickCommand(cmd: QuickCommand) {
    if (activePane?.ws?.readyState === WebSocket.OPEN) {
      activePane.ws.send(cmd.command + "\n");
      activePane.terminal?.focus();
    }
    showQuickCommands = false;
  }

  // 添加快捷命令
  function addQuickCommand() {
    if (!newCommandName.trim() || !newCommandText.trim()) return;
    
    const newCmd: QuickCommand = {
      id: Date.now().toString(),
      name: newCommandName.trim(),
      command: newCommandText.trim(),
      icon: "mdi:console-line",
    };
    
    quickCommands = [...quickCommands, newCmd];
    saveQuickCommands();
    newCommandName = "";
    newCommandText = "";
  }

  // 删除快捷命令
  function deleteQuickCommand(id: string) {
    quickCommands = quickCommands.filter(c => c.id !== id);
    saveQuickCommands();
  }

  // ==================== SSH 连接管理 ====================

  // 加载SSH连接列表
  async function loadSSHConnections() {
    const response = await sshService.listConnections();
    if (response.success && response.data) {
      sshConnections = response.data;
    }
  }

  // 打开新建SSH连接对话框
  function openNewSSHModal() {
    editingConnection = null;
    sshForm = {
      name: "",
      host: "",
      port: 22,
      username: "",
      password: "",
      auth_type: "password",
      private_key: "",
      passphrase: "",
      group: "",
      description: "",
    };
    sshError = "";
    showSSHModal = true;
  }

  // 打开编辑SSH连接对话框
  function openEditSSHModal(conn: SSHConnection) {
    editingConnection = conn;
    sshForm = {
      name: conn.name,
      host: conn.host,
      port: conn.port,
      username: conn.username,
      password: "", // 不回显密码
      auth_type: conn.auth_type,
      private_key: conn.private_key || "",
      passphrase: "",
      group: conn.group || "",
      description: conn.description || "",
    };
    sshError = "";
    showSSHModal = true;
  }

  // 关闭SSH对话框
  function closeSSHModal() {
    showSSHModal = false;
    editingConnection = null;
    sshError = "";
  }

  // 测试SSH连接
  async function testSSHConnection() {
    sshTesting = true;
    sshError = "";
    try {
      const response = await sshService.testConnection({
        host: sshForm.host,
        port: sshForm.port,
        username: sshForm.username,
        password: sshForm.auth_type === "password" ? sshForm.password : undefined,
        auth_type: sshForm.auth_type,
        private_key: sshForm.auth_type === "key" ? sshForm.private_key : undefined,
        passphrase: sshForm.auth_type === "key" ? sshForm.passphrase : undefined,
      });
      if (response.success) {
        sshError = "✓ " + $t("terminal.connectionSuccess");
      } else {
        sshError = response.message || $t("terminal.connectionFailed");
      }
    } catch (e: any) {
      sshError = e.message || $t("terminal.testConnectionFailed");
    } finally {
      sshTesting = false;
    }
  }

  // 保存SSH连接
  async function saveSSHConnection() {
    if (!sshForm.name.trim() || !sshForm.host.trim() || !sshForm.username.trim()) {
      sshError = $t("terminal.fillRequiredFields");
      return;
    }

    sshSaving = true;
    sshError = "";
    try {
      let response;
      if (editingConnection) {
        response = await sshService.updateConnection(editingConnection.id, {
          name: sshForm.name,
          host: sshForm.host,
          port: sshForm.port,
          username: sshForm.username,
          password: sshForm.password || undefined,
          auth_type: sshForm.auth_type,
          private_key: sshForm.auth_type === "key" ? sshForm.private_key : undefined,
          passphrase: sshForm.auth_type === "key" ? sshForm.passphrase : undefined,
          group: sshForm.group || undefined,
          description: sshForm.description || undefined,
        });
      } else {
        response = await sshService.createConnection({
          name: sshForm.name,
          host: sshForm.host,
          port: sshForm.port,
          username: sshForm.username,
          password: sshForm.auth_type === "password" ? sshForm.password : undefined,
          auth_type: sshForm.auth_type,
          private_key: sshForm.auth_type === "key" ? sshForm.private_key : undefined,
          passphrase: sshForm.auth_type === "key" ? sshForm.passphrase : undefined,
          group: sshForm.group || undefined,
          description: sshForm.description || undefined,
        });
      }

      if (response.success) {
        await loadSSHConnections();
        closeSSHModal();
      } else {
        sshError = response.message || $t("terminal.saveFailed");
      }
    } catch (e: any) {
      sshError = e.message || $t("terminal.saveFailed");
    } finally {
      sshSaving = false;
    }
  }

  // 删除SSH连接
  async function deleteSSHConnection(id: string) {
    if (!confirm($t("terminal.confirmDeleteConnection"))) return;
    
    try {
      const response = await sshService.deleteConnection(id);
      if (response.success) {
        await loadSSHConnections();
      }
    } catch (e) {
      console.error("删除SSH连接失败", e);
    }
  }

  // 连接SSH（创建新标签）
  async function connectSSH(conn: SSHConnection) {
    showSSHPanel = false;
    await createSSHTab(conn);
  }

  // 打开 SFTP 文件浏览器
  function openSFTPBrowser() {
    if (!activePane || activePane.connectionType !== 'ssh' || !activePane.sessionId) {
      return;
    }
    sftpSessionId = activePane.sessionId;
    sftpConnectionName = activePane.sshConnectionName || 'SSH';
    showSFTPBrowser = true;
  }

  // 关闭 SFTP 文件浏览器
  function closeSFTPBrowser() {
    showSFTPBrowser = false;
    sftpSessionId = "";
    sftpConnectionName = "";
  }

  // 派生当前主题（跟随系统）
  let currentTheme = $derived(systemTheme.isDark ? "dark" : "light");

  // 终端主题配置 - Catppuccin
  const themes = {
    dark: {
      background: "#1e1e2e",
      foreground: "#cdd6f4",
      cursor: "#f5e0dc",
      cursorAccent: "#1e1e2e",
      selectionBackground: "#45475a",
      selectionForeground: "#cdd6f4",
      selectionInactiveBackground: "#313244",
      black: "#45475a",
      red: "#f38ba8",
      green: "#a6e3a1",
      yellow: "#f9e2af",
      blue: "#89b4fa",
      magenta: "#f5c2e7",
      cyan: "#94e2d5",
      white: "#bac2de",
      brightBlack: "#585b70",
      brightRed: "#f38ba8",
      brightGreen: "#a6e3a1",
      brightYellow: "#f9e2af",
      brightBlue: "#89b4fa",
      brightMagenta: "#f5c2e7",
      brightCyan: "#94e2d5",
      brightWhite: "#a6adc8",
    },
    light: {
      background: "#eff1f5",
      foreground: "#4c4f69",
      cursor: "#dc8a78",
      cursorAccent: "#eff1f5",
      selectionBackground: "#7287fd",
      selectionForeground: "#eff1f5",
      selectionInactiveBackground: "#bcc0cc",
      black: "#5c5f77",
      red: "#d20f39",
      green: "#40a02b",
      yellow: "#df8e1d",
      blue: "#1e66f5",
      magenta: "#ea76cb",
      cyan: "#179299",
      white: "#acb0be",
      brightBlack: "#6c6f85",
      brightRed: "#d20f39",
      brightGreen: "#40a02b",
      brightYellow: "#df8e1d",
      brightBlue: "#1e66f5",
      brightMagenta: "#ea76cb",
      brightCyan: "#179299",
      brightWhite: "#bcc0cc",
    },
  } as const;

  type ThemeKey = keyof typeof themes;

  // 初始化 - 创建第一个标签页
  async function initTerminal() {
    await loadSSHConnections();
    await createTab(initialPath);
  }

  // 创建新的终端 pane
  function createPane(connectionType: ConnectionType = 'local', sshConnection?: SSHConnection): TerminalPane {
    const paneId = `pane-${++paneCounter}`;
    const newPane: TerminalPane = {
      id: paneId,
      terminal: null,
      fitAddon: null,
      searchAddon: null,
      sessionId: "",
      connected: false,
      connecting: false,
      ws: null,
      element: null,
      reconnectAttempts: 0,
      reconnectTimer: null,
      shouldReconnect: true,
      connectionType,
      sshConnectionId: sshConnection?.id,
      sshConnectionName: sshConnection?.name,
    };
    panes = [...panes, newPane];
    return newPane;
  }

  // 创建新标签页（本地终端）
  async function createTab(initialDir?: string) {
    const tabId = `tab-${++tabCounter}`;
    const pane = createPane('local');
    
    const newTab: TerminalTab = {
      id: tabId,
      title: `${$t("terminal.tabTitle")} ${tabCounter}`,
      layout: {
        id: `node-${tabId}-root`,
        type: 'pane',
        paneId: pane.id,
      },
      activePaneId: pane.id,
      connectionType: 'local',
    };

    tabs = [...tabs, newTab];
    activeTabId = tabId;

    // 等待 DOM 更新
    await new Promise(r => requestAnimationFrame(r));
    
    // 初始化 pane 的终端
    await initPaneTerminal(pane.id, initialDir);
  }

  // 创建SSH标签页
  async function createSSHTab(conn: SSHConnection) {
    const tabId = `tab-${++tabCounter}`;
    const pane = createPane('ssh', conn);
    
    const newTab: TerminalTab = {
      id: tabId,
      title: `${conn.name}`,
      layout: {
        id: `node-${tabId}-root`,
        type: 'pane',
        paneId: pane.id,
      },
      activePaneId: pane.id,
      connectionType: 'ssh',
      sshConnectionId: conn.id,
    };

    tabs = [...tabs, newTab];
    activeTabId = tabId;

    // 等待 DOM 更新
    await new Promise(r => requestAnimationFrame(r));
    
    // 初始化 pane 的终端
    await initPaneTerminal(pane.id);
  }

  // 初始化 pane 的终端
  async function initPaneTerminal(paneId: string, initialDir?: string) {
    const pane = panes.find(p => p.id === paneId);
    if (!pane || !pane.element) return;

    try {
      const { Terminal } = await import("@xterm/xterm");
      const { FitAddon } = await import("@xterm/addon-fit");
      const { WebLinksAddon } = await import("@xterm/addon-web-links");
      const { SearchAddon } = await import("@xterm/addon-search");

      const terminal = new Terminal({
        fontSize,
        fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, Monaco, monospace',
        cursorBlink: true,
        cursorStyle: "block",
        theme: themes[currentTheme as ThemeKey],
        allowProposedApi: true,
        scrollback: 10000,
        convertEol: true,
      });

      const fitAddon = new FitAddon();
      terminal.loadAddon(fitAddon);
      terminal.loadAddon(new WebLinksAddon());
      const searchAddon = new SearchAddon();
      terminal.loadAddon(searchAddon);

      terminal.open(pane.element);

      pane.terminal = terminal;
      pane.fitAddon = fitAddon;
      pane.searchAddon = searchAddon;

      requestAnimationFrame(() => {
        fitAddon.fit();
      });

      // 设置剪贴板和快捷键
      setupPaneClipboard(pane);

      // 连接到后端
      await connectPane(pane, initialDir);

      terminal.focus();
    } catch (err) {
      console.error("初始化终端失败", err);
      if (pane.element) {
        pane.element.innerHTML = `
          <div style="padding: 20px; color: #f38ba8; font-family: monospace;">
            <p>⚠ 终端组件加载失败</p>
          </div>
        `;
      }
    }
  }

  // 设置 pane 剪贴板功能
  function setupPaneClipboard(pane: TerminalPane) {
    if (!pane.terminal) return;

    pane.terminal.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if (e.ctrlKey && e.shiftKey && e.key === "C") {
        e.preventDefault();
        copySelection();
        return false;
      }
      if (e.ctrlKey && e.shiftKey && e.key === "V") {
        e.preventDefault();
        pasteFromClipboard();
        return false;
      }
      if (e.ctrlKey && !e.shiftKey && e.key === "f") {
        e.preventDefault();
        toggleSearch();
        return false;
      }
      if (e.key === "Escape" && showSearch) {
        closeSearch();
        return false;
      }
      // Ctrl+T: 新建标签页
      if (e.ctrlKey && e.shiftKey && e.key === "T") {
        e.preventDefault();
        createTab();
        return false;
      }
      // Ctrl+W: 关闭当前 pane（如果只有一个则关闭标签页）
      if (e.ctrlKey && e.shiftKey && e.key === "W") {
        e.preventDefault();
        closeCurrentPane();
        return false;
      }
      // Ctrl+Shift+D: 水平分屏（上下）
      if (e.ctrlKey && e.shiftKey && e.key === "D") {
        e.preventDefault();
        splitPane('horizontal');
        return false;
      }
      // Ctrl+Shift+E: 垂直分屏（左右）
      if (e.ctrlKey && e.shiftKey && e.key === "E") {
        e.preventDefault();
        splitPane('vertical');
        return false;
      }
      // Alt+方向键: 切换焦点到相邻 pane
      if (e.altKey && ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(e.key)) {
        e.preventDefault();
        navigatePane(e.key.replace("Arrow", "").toLowerCase() as 'up' | 'down' | 'left' | 'right');
        return false;
      }
      return true;
    });
  }

  // 复制选中文本
  async function copySelection() {
    if (!activePane?.terminal) return;
    const selection = activePane.terminal.getSelection();
    if (selection) {
      try {
        await navigator.clipboard.writeText(selection);
        activePane.terminal.select(0, 0, 0);
      } catch (err) {
        console.error("复制失败", err);
      }
    }
  }

  // 从剪贴板粘贴
  async function pasteFromClipboard() {
    try {
      const text = await navigator.clipboard.readText();
      if (text && activePane?.ws?.readyState === WebSocket.OPEN) {
        activePane.ws.send(text);
      }
    } catch (err) {
      console.error("粘贴失败", err);
    }
  }

  // 搜索功能
  function toggleSearch() {
    showSearch = !showSearch;
    if (showSearch) {
      requestAnimationFrame(() => {
        searchInputRef?.focus();
        searchInputRef?.select();
      });
    } else {
      activePane?.terminal?.focus();
    }
  }

  function closeSearch() {
    showSearch = false;
    searchQuery = "";
    activePane?.searchAddon?.clearDecorations();
    activePane?.terminal?.focus();
  }

  function searchNext() {
    if (searchQuery && activePane?.searchAddon) {
      activePane.searchAddon.findNext(searchQuery, { caseSensitive: false, regex: false });
    }
  }

  function searchPrevious() {
    if (searchQuery && activePane?.searchAddon) {
      activePane.searchAddon.findPrevious(searchQuery, { caseSensitive: false, regex: false });
    }
  }

  function handleSearchKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      if (e.shiftKey) {
        searchPrevious();
      } else {
        searchNext();
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      closeSearch();
    }
  }

  // 右键菜单
  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    
    // 计算菜单位置，确保不超出容器
    const container = activePane?.element?.parentElement;
    if (container) {
      const rect = container.getBoundingClientRect();
      let x = e.clientX - rect.left;
      let y = e.clientY - rect.top;
      
      // 预估菜单尺寸，防止超出边界
      const menuWidth = 180;
      const menuHeight = 200;
      if (x + menuWidth > rect.width) x = rect.width - menuWidth - 8;
      if (y + menuHeight > rect.height) y = rect.height - menuHeight - 8;
      
      contextMenuPos = { x: Math.max(8, x), y: Math.max(8, y) };
    } else {
      contextMenuPos = { x: e.offsetX, y: e.offsetY };
    }
    
    showContextMenu = true;
  }

  function closeContextMenu() {
    showContextMenu = false;
  }

  function contextMenuAction(action: () => void) {
    action();
    closeContextMenu();
    activePane?.terminal?.focus();
  }

  // 连接到终端会话
  async function connectPane(pane: TerminalPane, initialDir?: string) {
    if (pane.connecting || pane.connected) return;

    pane.connecting = true;

    try {
      const cols = pane.terminal?.cols || 80;
      const rows = pane.terminal?.rows || 24;

      let sessionId: string;
      let wsUrl: string;

      if (pane.connectionType === 'ssh' && pane.sshConnectionId) {
        // SSH 连接
        pane.terminal?.write(`\x1b[36m正在连接到 ${pane.sshConnectionName || 'SSH'}...\x1b[0m\r\n`);
        const response = await sshService.createSession(pane.sshConnectionId, cols, rows);
        if (!response.success || !response.data) {
          throw new Error(response.message || $t('terminal.createSSHSessionFailed'));
        }
        sessionId = response.data.id;
        wsUrl = sshService.getWebSocketUrl(sessionId);
      } else {
        // 本地终端
        const response = await terminalService.createSession(cols, rows);
        if (!response.success || !response.data) {
          throw new Error(response.message || $t('terminal.createTerminalSessionFailed'));
        }
        sessionId = response.data.id;
        wsUrl = terminalService.getWebSocketUrl(sessionId);
      }

      pane.sessionId = sessionId;
      pane.ws = new WebSocket(wsUrl);
      pane.ws.binaryType = "arraybuffer";

      pane.ws.onopen = () => {
        pane.connected = true;
        pane.connecting = false;
        pane.reconnectAttempts = 0;
        pane.terminal?.focus();
        
        // 如果指定了初始目录，发送 cd 命令
        if (initialDir && pane.ws?.readyState === WebSocket.OPEN) {
          // 使用延时确保终端已准备好
          setTimeout(() => {
            if (pane.ws?.readyState === WebSocket.OPEN) {
              pane.ws.send(`cd ${JSON.stringify(initialDir)} && clear\n`);
            }
          }, 100);
        }
      };

      pane.ws.onmessage = (event) => {
        if (pane.terminal) {
          if (event.data instanceof ArrayBuffer) {
            const decoder = new TextDecoder();
            pane.terminal.write(decoder.decode(event.data));
          } else {
            pane.terminal.write(event.data);
          }
        }
      };

      pane.ws.onclose = () => {
        pane.connected = false;
        if (pane.shouldReconnect && pane.reconnectAttempts < RECONNECT_MAX_ATTEMPTS) {
          const delay = RECONNECT_BASE_DELAY * Math.pow(2, pane.reconnectAttempts);
          pane.reconnectAttempts++;
          pane.terminal?.write(`\r\n\x1b[33m[连接已断开，${delay/1000}秒后重试 (${pane.reconnectAttempts}/${RECONNECT_MAX_ATTEMPTS})...]\x1b[0m\r\n`);
          pane.reconnectTimer = window.setTimeout(() => {
            pane.reconnectTimer = null;
            if (pane.shouldReconnect) {
              connectPane(pane);
            }
          }, delay);
        } else if (pane.reconnectAttempts >= RECONNECT_MAX_ATTEMPTS) {
          pane.terminal?.write("\r\n\x1b[31m[重连失败，请手动重新连接]\x1b[0m\r\n");
        } else {
          pane.terminal?.write("\r\n\x1b[33m[连接已断开]\x1b[0m\r\n");
        }
      };

      pane.ws.onerror = (error) => {
        console.error("WebSocket 错误", error);
        pane.connected = false;
        pane.terminal?.write("\r\n\x1b[31m[连接错误]\x1b[0m\r\n");
      };

      pane.terminal?.onData((data: string) => {
        if (pane.ws?.readyState === WebSocket.OPEN) {
          pane.ws.send(data);
        }
      });

      pane.terminal?.onResize(({ cols, rows }: { cols: number; rows: number }) => {
        if (pane.ws?.readyState === WebSocket.OPEN) {
          pane.ws.send(JSON.stringify({ type: "resize", cols, rows }));
        }
      });
    } catch (err: any) {
      console.error("连接终端失败", err);
      pane.terminal?.write(`\x1b[31m连接失败: ${err.message}\x1b[0m\r\n`);
    } finally {
      pane.connecting = false;
    }
  }

  // 断开指定 pane 连接
  function disconnectPane(pane: TerminalPane, allowReconnect = false) {
    if (pane.reconnectTimer) {
      clearTimeout(pane.reconnectTimer);
      pane.reconnectTimer = null;
    }
    pane.shouldReconnect = allowReconnect;
    pane.reconnectAttempts = 0;
    
    if (pane.ws) {
      pane.ws.close();
      pane.ws = null;
    }
    if (pane.sessionId) {
      // 根据连接类型调用不同的关闭服务
      if (pane.connectionType === 'ssh') {
        sshService.closeSession(pane.sessionId).catch(console.error);
      } else {
        terminalService.closeSession(pane.sessionId).catch(console.error);
      }
      pane.sessionId = "";
    }
    pane.connected = false;
  }

  // 断开当前 pane
  function disconnect() {
    if (activePane) {
      disconnectPane(activePane, false);
      activePane.terminal?.write("\r\n\x1b[33m[已手动断开连接]\x1b[0m\r\n");
    }
  }

  // 重新连接当前 pane
  async function reconnect() {
    if (activePane) {
      disconnectPane(activePane, false);
      activePane.terminal?.clear();
      activePane.shouldReconnect = true;
      await connectPane(activePane);
    }
  }

  // 遍历布局树，获取所有 pane ID
  function getAllPaneIds(node: SplitNode): string[] {
    if (node.type === 'pane') {
      return node.paneId ? [node.paneId] : [];
    }
    const ids: string[] = [];
    if (node.first) ids.push(...getAllPaneIds(node.first));
    if (node.second) ids.push(...getAllPaneIds(node.second));
    return ids;
  }

  // 关闭标签页
  function closeTab(tabId: string) {
    const tab = tabs.find(t => t.id === tabId);
    if (!tab) return;

    // 清理所有 pane
    const paneIds = getAllPaneIds(tab.layout);
    for (const paneId of paneIds) {
      const pane = panes.find(p => p.id === paneId);
      if (pane) {
        disconnectPane(pane);
        pane.terminal?.dispose();
      }
    }
    panes = panes.filter(p => !paneIds.includes(p.id));

    // 从列表中移除
    const index = tabs.findIndex(t => t.id === tabId);
    tabs = tabs.filter(t => t.id !== tabId);

    // 如果关闭的是当前标签页，切换到另一个
    if (activeTabId === tabId && tabs.length > 0) {
      const newIndex = Math.min(index, tabs.length - 1);
      switchTab(tabs[newIndex].id);
    }

    // 如果没有标签页了，创建一个新的
    if (tabs.length === 0) {
      createTab();
    }
  }

  // 切换标签页
  function switchTab(tabId: string) {
    activeTabId = tabId;
    requestAnimationFrame(() => {
      activePane?.terminal?.focus();
      activePane?.fitAddon?.fit();
    });
  }

  // 调整字体大小
  function setFontSize(size: number) {
    fontSize = Math.max(10, Math.min(24, size));
    // 更新所有 pane 的字体大小
    for (const pane of panes) {
      if (pane.terminal) {
        pane.terminal.options.fontSize = fontSize;
        requestAnimationFrame(() => {
          pane.fitAddon?.fit();
        });
      }
    }
  }

  // 监听系统主题变化，更新所有终端主题
  $effect(() => {
    for (const pane of panes) {
      if (pane.terminal) {
        pane.terminal.options.theme = themes[currentTheme as ThemeKey];
      }
    }
  });

  // 清屏
  function clearTerminal() {
    activePane?.terminal?.clear();
    activePane?.terminal?.focus();
  }

  // 点击容器时聚焦终端
  function focusTerminal() {
    activePane?.terminal?.focus();
  }

  // 分屏功能
  function splitPane(direction: SplitDirection) {
    if (!activeTab || !activePane) return;

    const newPane = createPane();
    const currentPaneId = activePane.id;

    // 在布局树中找到当前 pane 并分裂
    function splitNode(node: SplitNode): SplitNode {
      if (node.type === 'pane' && node.paneId === currentPaneId) {
        return {
          id: `split-${Date.now()}`,
          type: 'split',
          direction,
          ratio: 0.5,
          first: { ...node },
          second: {
            id: `node-${Date.now()}`,
            type: 'pane',
            paneId: newPane.id
          }
        };
      }
      if (node.type === 'split') {
        return {
          ...node,
          first: node.first ? splitNode(node.first) : node.first,
          second: node.second ? splitNode(node.second) : node.second
        };
      }
      return node;
    }

    activeTab.layout = splitNode(activeTab.layout);
    activeTab.activePaneId = newPane.id;

    // 初始化新 pane
    requestAnimationFrame(() => {
      initPaneTerminal(newPane.id);
    });
  }

  // 关闭当前 pane（如果是最后一个则关闭 tab）
  function closeCurrentPane() {
    if (!activeTab || !activePane) return;

    const paneIds = getAllPaneIds(activeTab.layout);
    if (paneIds.length <= 1) {
      // 只有一个 pane，关闭整个 tab
      closeTab(activeTab.id);
      return;
    }

    const closingPaneId = activePane.id;
    
    // 从布局中移除 pane，并找到兄弟节点替代
    function removePane(node: SplitNode, parent: SplitNode | null, isFirst: boolean): SplitNode | null {
      if (node.type === 'pane' && node.paneId === closingPaneId) {
        return null; // 标记为删除
      }
      if (node.type === 'split') {
        const newFirst = node.first ? removePane(node.first, node, true) : null;
        const newSecond = node.second ? removePane(node.second, node, false) : null;

        if (newFirst === null && newSecond) {
          return newSecond; // 用 second 替代整个 split
        }
        if (newSecond === null && newFirst) {
          return newFirst; // 用 first 替代整个 split
        }
        if (newFirst && newSecond) {
          return { ...node, first: newFirst, second: newSecond };
        }
      }
      return node;
    }

    const newLayout = removePane(activeTab.layout, null, true);
    if (newLayout) {
      activeTab.layout = newLayout;
    }

    // 清理 pane
    const pane = panes.find(p => p.id === closingPaneId);
    if (pane) {
      disconnectPane(pane);
      pane.terminal?.dispose();
      panes = panes.filter(p => p.id !== closingPaneId);
    }

    // 切换到第一个可用的 pane
    const remainingIds = getAllPaneIds(activeTab.layout);
    if (remainingIds.length > 0) {
      activeTab.activePaneId = remainingIds[0];
      requestAnimationFrame(() => {
        activePane?.terminal?.focus();
      });
    }
  }

  // 在 pane 之间导航
  function navigatePane(direction: 'up' | 'down' | 'left' | 'right') {
    if (!activeTab || !activePane) return;

    const paneIds = getAllPaneIds(activeTab.layout);
    if (paneIds.length <= 1) return;

    // 简化实现：循环切换到下一个 pane
    const currentIndex = paneIds.indexOf(activePane.id);
    let nextIndex: number;

    if (direction === 'right' || direction === 'down') {
      nextIndex = (currentIndex + 1) % paneIds.length;
    } else {
      nextIndex = (currentIndex - 1 + paneIds.length) % paneIds.length;
    }

    activeTab.activePaneId = paneIds[nextIndex];
    requestAnimationFrame(() => {
      activePane?.terminal?.focus();
    });
  }

  // 分屏拖拽调整大小
  function onSplitDragStart(e: MouseEvent, node: SplitNode) {
    if (node.type !== 'split') return;
    e.preventDefault();
    draggingSplit = node.id;

    const startPos = node.direction === 'horizontal' ? e.clientY : e.clientX;
    const startRatio = node.ratio || 0.5;

    const onMove = (moveEvent: MouseEvent) => {
      const container = (e.target as HTMLElement).parentElement;
      if (!container) return;

      const rect = container.getBoundingClientRect();
      const size = node.direction === 'horizontal' ? rect.height : rect.width;
      const pos = node.direction === 'horizontal' ? moveEvent.clientY : moveEvent.clientX;
      const offset = node.direction === 'horizontal' ? rect.top : rect.left;

      const newRatio = Math.max(0.1, Math.min(0.9, (pos - offset) / size));
      node.ratio = newRatio;

      // 触发重新渲染
      tabs = [...tabs];
      
      // 调整所有 pane 的大小
      requestAnimationFrame(() => {
        for (const pane of panes) {
          pane.fitAddon?.fit();
        }
      });
    };

    const onUp = () => {
      draggingSplit = null;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };

    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
  }

  onMount(() => {
    initQuickCommands();
    initTerminal();
  });

  onDestroy(() => {
    // 清理所有 pane
    for (const pane of panes) {
      disconnectPane(pane);
      pane.terminal?.dispose();
    }
  });
</script>

<div class="terminal-app" class:dark={currentTheme === "dark"} class:light={currentTheme === "light"}>
  <!-- 标签栏 -->
  <div class="tab-bar">
    <div class="tabs">
      {#each tabs as tab (tab.id)}
        {@const tabPane = panes.find(p => p.id === tab.activePaneId)}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="tab"
          class:active={tab.id === activeTabId}
          onclick={() => switchTab(tab.id)}
          role="tab"
          tabindex="0"
          onkeydown={(e) => e.key === 'Enter' && switchTab(tab.id)}
        >
          {#if tab.connectionType === 'ssh'}
            <Icon icon="mdi:server-network" class="tab-icon ssh" />
          {:else}
            <Icon icon="mdi:console" class="tab-icon" />
          {/if}
          <span class="tab-status" class:connected={tabPane?.connected} class:connecting={tabPane?.connecting}></span>
          <span class="tab-title">{tab.title}</span>
          {#if tabs.length > 1}
            <button
              class="tab-close"
              onclick={(e) => { e.stopPropagation(); closeTab(tab.id); }}
              title={$t("terminal.closeTab")}
            >
              <Icon icon="mdi:close" />
            </button>
          {/if}
        </div>
      {/each}
    </div>
    <button class="new-tab" onclick={createTab} title={$t("terminal.newLocalTerminal")}>
      <Icon icon="mdi:plus" />
    </button>
    <button class="new-ssh" onclick={() => showSSHPanel = !showSSHPanel} title={$t("terminal.sshConnection")} class:active={showSSHPanel}>
      <Icon icon="mdi:server-network" />
    </button>
  </div>

  <!-- 工具栏 -->
  <div class="toolbar">
    <div class="toolbar-left">
      <span class="status" class:connected={activePane?.connected} class:connecting={activePane?.connecting}>
        <span class="status-dot"></span>
        {#if activePane?.connecting}
          {$t("terminal.connecting")}
        {:else if activePane?.connected}
          {$t("terminal.connected")}
        {:else}
          {$t("terminal.disconnected")}
        {/if}
      </span>
    </div>
    <div class="toolbar-right">
      <button onclick={() => setFontSize(fontSize - 1)} title={$t("terminal.decreaseFont")}>
        <Icon icon="mdi:format-font-size-decrease" />
      </button>
      <span class="font-size">{fontSize}px</span>
      <button onclick={() => setFontSize(fontSize + 1)} title={$t("terminal.increaseFont")}>
        <Icon icon="mdi:format-font-size-increase" />
      </button>
      <div class="divider"></div>
      <button onclick={copySelection} title={$t("terminal.copy")}>
        <Icon icon="mdi:content-copy" />
      </button>
      <button onclick={pasteFromClipboard} title={$t("terminal.paste")}>
        <Icon icon="mdi:content-paste" />
      </button>
      <button onclick={toggleSearch} title={$t("terminal.search")} class:active={showSearch}>
        <Icon icon="mdi:magnify" />
      </button>
      <button onclick={() => showQuickCommands = !showQuickCommands} title={$t("terminal.quickCommands")} class:active={showQuickCommands}>
        <Icon icon="mdi:lightning-bolt" />
      </button>
      <div class="divider"></div>
      <button onclick={clearTerminal} title={$t("terminal.clearScreen")}>
        <Icon icon="mdi:text-box-remove" />
      </button>
      {#if activePane?.connectionType === 'ssh' && activePane?.connected}
        <button onclick={openSFTPBrowser} title={$t("terminal.sftpBrowser")} class="sftp-btn">
          <Icon icon="mdi:folder-network" />
        </button>
      {/if}
      <div class="divider"></div>
      {#if activePane?.connected}
        <button onclick={disconnect} title={$t("terminal.disconnect")} class="danger">
          <Icon icon="mdi:close-circle" />
        </button>
      {:else}
        <button onclick={reconnect} title={$t("terminal.reconnect")} class="success">
          <Icon icon="mdi:refresh" />
        </button>
      {/if}
      <div class="divider"></div>
      <button onclick={() => splitPane('horizontal')} title={$t("terminal.splitHorizontal")}>
        <Icon icon="mdi:arrow-split-horizontal" />
      </button>
      <button onclick={() => splitPane('vertical')} title={$t("terminal.splitVertical")}>
        <Icon icon="mdi:arrow-split-vertical" />
      </button>
    </div>
  </div>

  <!-- 搜索框 -->
  {#if showSearch}
    <div class="search-bar">
      <Icon icon="mdi:magnify" class="search-icon" />
      <input
        type="text"
        bind:this={searchInputRef}
        bind:value={searchQuery}
        onkeydown={handleSearchKeydown}
        oninput={searchNext}
        placeholder={$t("terminal.searchPlaceholder")}
      />
      <button onclick={searchPrevious} title={$t("terminal.previous")}>
        <Icon icon="mdi:chevron-up" />
      </button>
      <button onclick={searchNext} title={$t("terminal.next")}>
        <Icon icon="mdi:chevron-down" />
      </button>
      <button onclick={closeSearch} title={$t("terminal.close")}>
        <Icon icon="mdi:close" />
      </button>
    </div>
  {/if}

  <!-- SSH 连接面板 -->
  {#if showSSHPanel}
    <div class="ssh-panel">
      <div class="ssh-panel-header">
        <span>{$t("terminal.sshConnectionPanel")}</span>
        <div class="ssh-panel-actions">
          <button onclick={openNewSSHModal} title={$t("terminal.newConnection")}>
            <Icon icon="mdi:plus" />
          </button>
          <button onclick={() => loadSSHConnections()} title={$t("terminal.refresh")}>
            <Icon icon="mdi:refresh" />
          </button>
          <button onclick={() => showSSHPanel = false} title={$t("terminal.close")}>
            <Icon icon="mdi:close" />
          </button>
        </div>
      </div>
      <div class="ssh-list">
        {#if sshConnections.length === 0}
          <div class="ssh-empty">
            <Icon icon="mdi:server-off" />
            <span>{$t("terminal.noConnections")}</span>
            <button class="add-btn" onclick={openNewSSHModal}>
              <Icon icon="mdi:plus" /> {$t("terminal.addConnection")}
            </button>
          </div>
        {:else}
          {#each sshConnections as conn (conn.id)}
            <div class="ssh-item">
              <button class="ssh-connect-btn" onclick={() => connectSSH(conn)} title={$t("terminal.clickToConnect")}>
                <Icon icon="mdi:server-network" />
                <div class="ssh-info">
                  <span class="ssh-name">{conn.name}</span>
                  <span class="ssh-host">{conn.username}@{conn.host}:{conn.port}</span>
                </div>
              </button>
              <div class="ssh-item-actions">
                <button onclick={() => openEditSSHModal(conn)} title={$t("terminal.edit")}>
                  <Icon icon="mdi:pencil" />
                </button>
                <button onclick={() => deleteSSHConnection(conn.id)} title={$t("terminal.delete")} class="danger">
                  <Icon icon="mdi:delete" />
                </button>
              </div>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}

  <!-- SSH 连接编辑对话框 -->
  {#if showSSHModal}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="modal-overlay" onclick={() => { sshShaking = true; setTimeout(() => sshShaking = false, 500); }} onkeydown={(e) => e.key === 'Escape' && closeSSHModal()}>
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="ssh-modal" class:shake={sshShaking} onclick={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <h3>{editingConnection ? $t("terminal.editConnection") : $t("terminal.newSSHConnection")}</h3>
          <button onclick={closeSSHModal}>
            <Icon icon="mdi:close" />
          </button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label for="ssh-name">{$t("terminal.name")}</label>
            <input id="ssh-name" type="text" bind:value={sshForm.name} placeholder={$t("terminal.namePlaceholder")} />
          </div>
          <div class="form-row">
            <div class="form-group flex-1">
              <label for="ssh-host">{$t("terminal.host")}</label>
              <input id="ssh-host" type="text" bind:value={sshForm.host} placeholder="192.168.1.100" />
            </div>
            <div class="form-group port">
              <label for="ssh-port">{$t("terminal.port")}</label>
              <input id="ssh-port" type="number" bind:value={sshForm.port} min="1" max="65535" />
            </div>
          </div>
          <div class="form-group">
            <label for="ssh-username">{$t("terminal.username")}</label>
            <input id="ssh-username" type="text" bind:value={sshForm.username} placeholder="root" />
          </div>
          <div class="form-group">
            <label>{$t("terminal.authMethod")}</label>
            <div class="auth-type-tabs">
              <button 
                class:active={sshForm.auth_type === 'password'} 
                onclick={() => sshForm.auth_type = 'password'}
              >
                {$t("terminal.password")}
              </button>
              <button 
                class:active={sshForm.auth_type === 'key'} 
                onclick={() => sshForm.auth_type = 'key'}
              >
                {$t("terminal.privateKey")}
              </button>
            </div>
          </div>
          {#if sshForm.auth_type === 'password'}
            <div class="form-group">
              <label for="ssh-password">{$t("terminal.password")}</label>
              <input id="ssh-password" type="password" bind:value={sshForm.password} placeholder={$t("terminal.passwordPlaceholder")} />
            </div>
          {:else}
            <div class="form-group">
              <label for="ssh-key">{$t("terminal.privateKey")}</label>
              <textarea id="ssh-key" bind:value={sshForm.private_key} placeholder={$t("terminal.privateKeyPlaceholder")} rows="4"></textarea>
            </div>
            <div class="form-group">
              <label for="ssh-passphrase">{$t("terminal.passphrase")}</label>
              <input id="ssh-passphrase" type="password" bind:value={sshForm.passphrase} placeholder={$t("terminal.passphrasePlaceholder")} />
            </div>
          {/if}
          <div class="form-group">
            <label for="ssh-desc">{$t("terminal.description")}</label>
            <input id="ssh-desc" type="text" bind:value={sshForm.description} placeholder={$t("terminal.descriptionPlaceholder")} />
          </div>
          {#if sshError}
            <div class="form-error" class:success={sshError.startsWith('✓')}>
              {sshError}
            </div>
          {/if}
        </div>
        <div class="modal-footer">
          <button class="btn-secondary" onclick={testSSHConnection} disabled={sshTesting || !sshForm.host || !sshForm.username}>
            {sshTesting ? $t("terminal.testing") : $t("terminal.testConnection")}
          </button>
          <div class="flex-1"></div>
          <button class="btn-secondary" onclick={closeSSHModal}>{$t("terminal.cancel")}</button>
          <button class="btn-primary" onclick={saveSSHConnection} disabled={sshSaving}>
            {sshSaving ? $t("terminal.saving") : $t("terminal.save")}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- 快捷命令面板 -->
  {#if showQuickCommands}
    <div class="quick-commands-panel">
      <div class="quick-commands-header">
        <span>{$t("terminal.quickCommands")}</span>
        <button onclick={() => showQuickCommands = false} title={$t("terminal.close")}>
          <Icon icon="mdi:close" />
        </button>
      </div>
      <div class="quick-commands-list">
        {#each quickCommands as cmd (cmd.id)}
          <div class="quick-command-item">
            <button class="command-btn" onclick={() => executeQuickCommand(cmd)} title={cmd.command}>
              <Icon icon={cmd.icon || "mdi:console-line"} />
              <span class="command-name">{cmd.name}</span>
              <span class="command-text">{cmd.command}</span>
            </button>
            <button class="delete-btn" onclick={() => deleteQuickCommand(cmd.id)} title={$t("terminal.delete")}>
              <Icon icon="mdi:delete-outline" />
            </button>
          </div>
        {/each}
      </div>
      <div class="quick-commands-add">
        <input
          type="text"
          bind:value={newCommandName}
          placeholder={$t("terminal.quickCommandName")}
          class="name-input"
        />
        <input
          type="text"
          bind:value={newCommandText}
          placeholder={$t("terminal.quickCommandText")}
          class="command-input"
          onkeydown={(e) => e.key === "Enter" && addQuickCommand()}
        />
        <button onclick={addQuickCommand} title={$t("common.add")} disabled={!newCommandName.trim() || !newCommandText.trim()}>
          <Icon icon="mdi:plus" />
        </button>
      </div>
    </div>
  {/if}

  <!-- 终端容器 -->
  <div class="terminals-wrapper">
    {#each tabs as tab (tab.id)}
      <div
        class="tab-content"
        class:hidden={tab.id !== activeTabId}
      >
        {#snippet renderNode(node: SplitNode)}
          {#if node.type === 'pane'}
            {@const pane = panes.find(p => p.id === node.paneId)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
              class="terminal-pane"
              class:active={activePane?.id === node.paneId}
              data-pane-id={node.paneId}
              onclick={() => { 
                if (activeTab) activeTab.activePaneId = node.paneId || '';
                focusTerminal();
                closeContextMenu();
              }}
              oncontextmenu={handleContextMenu}
              onkeydown={(e) => e.stopPropagation()}
              onkeyup={(e) => e.stopPropagation()}
              onkeypress={(e) => e.stopPropagation()}
            >
              {#if pane}
                <div class="pane-terminal" bind:this={pane.element}></div>
              {/if}
            </div>
          {:else if node.type === 'split'}
            <div 
              class="split-container" 
              class:horizontal={node.direction === 'horizontal'}
              class:vertical={node.direction === 'vertical'}
            >
              {#if node.first}
                <div class="split-pane first" style="flex: {node.ratio || 0.5};">
                  {@render renderNode(node.first)}
                </div>
              {/if}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div 
                class="split-resizer"
                class:horizontal={node.direction === 'horizontal'}
                class:vertical={node.direction === 'vertical'}
                class:dragging={draggingSplit === node.id}
                onmousedown={(e) => onSplitDragStart(e, node)}
              ></div>
              {#if node.second}
                <div class="split-pane second" style="flex: {1 - (node.ratio || 0.5)};">
                  {@render renderNode(node.second)}
                </div>
              {/if}
            </div>
          {/if}
        {/snippet}
        {@render renderNode(tab.layout)}
      </div>
    {/each}

    <!-- 右键菜单 -->
    {#if showContextMenu}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="context-menu"
        style="left: {contextMenuPos.x}px; top: {contextMenuPos.y}px"
        onclick={(e) => e.stopPropagation()}
      >
        <button onclick={() => contextMenuAction(copySelection)}>
          <Icon icon="mdi:content-copy" />
          <span>{$t("terminal.copyShort")}</span>
          <span class="shortcut">Ctrl+Shift+C</span>
        </button>
        <button onclick={() => contextMenuAction(pasteFromClipboard)}>
          <Icon icon="mdi:content-paste" />
          <span>{$t("terminal.pasteShort")}</span>
          <span class="shortcut">Ctrl+Shift+V</span>
        </button>
        <div class="menu-divider"></div>
        <button onclick={() => contextMenuAction(toggleSearch)}>
          <Icon icon="mdi:magnify" />
          <span>{$t("terminal.searchShort")}</span>
          <span class="shortcut">Ctrl+F</span>
        </button>
        <button onclick={() => contextMenuAction(clearTerminal)}>
          <Icon icon="mdi:text-box-remove" />
          <span>{$t("terminal.clearScreen")}</span>
        </button>
        <div class="menu-divider"></div>
        <button onclick={() => contextMenuAction(createTab)}>
          <Icon icon="mdi:plus" />
          <span>{$t("terminal.newTab")}</span>
          <span class="shortcut">Ctrl+Shift+T</span>
        </button>
        <button onclick={() => contextMenuAction(() => splitPane('horizontal'))}>
          <Icon icon="mdi:arrow-split-horizontal" />
          <span>{$t("terminal.splitHorizontalShort")}</span>
          <span class="shortcut">Ctrl+Shift+D</span>
        </button>
        <button onclick={() => contextMenuAction(() => splitPane('vertical'))}>
          <Icon icon="mdi:arrow-split-vertical" />
          <span>{$t("terminal.splitVerticalShort")}</span>
          <span class="shortcut">Ctrl+Shift+E</span>
        </button>
        {#if getAllPaneIds(activeTab?.layout || { id: '', type: 'pane' }).length > 1}
          <button onclick={() => contextMenuAction(closeCurrentPane)}>
            <Icon icon="mdi:close" />
            <span>{$t("terminal.closeCurrentPane")}</span>
          </button>
        {/if}
        <div class="menu-divider"></div>
        {#if activePane?.connected}
          <button onclick={() => contextMenuAction(disconnect)} class="danger">
            <Icon icon="mdi:close-circle" />
            <span>{$t("terminal.disconnectShort")}</span>
          </button>
        {:else}
          <button onclick={() => contextMenuAction(reconnect)} class="success">
            <Icon icon="mdi:refresh" />
            <span>{$t("terminal.reconnectShort")}</span>
          </button>
        {/if}
      </div>
    {/if}
  </div>

  <!-- SFTP 文件浏览器 -->
  {#if showSFTPBrowser && sftpSessionId}
    <div class="sftp-overlay">
      <SFTPBrowser
        sessionId={sftpSessionId}
        connectionName={sftpConnectionName}
        onClose={closeSFTPBrowser}
      />
    </div>
  {/if}
</div>

<style>
  .terminal-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: #1e1e2e;
    position: relative;

    &.light {
      background: #eff1f5;

      .tab-bar {
        background: #e6e9ef;
        border-bottom-color: #ccd0da;

        .tab {
          color: #4c4f69;
          background: transparent;

          &:hover {
            background: rgba(0, 0, 0, 0.05);
          }

          &.active {
            background: #eff1f5;
            color: #4c4f69;
          }

          .tab-close:hover {
            background: rgba(0, 0, 0, 0.1);
          }
        }

        .new-tab {
          color: #4c4f69;

          &:hover {
            background: rgba(0, 0, 0, 0.05);
          }
        }
      }

      .toolbar {
        background: #e6e9ef;
        border-bottom-color: #ccd0da;
        color: #4c4f69;

        button {
          color: #4c4f69;

          &:hover {
            background: rgba(0, 0, 0, 0.1);
          }
        }
      }
    }
  }

  .tab-bar {
    display: flex;
    align-items: center;
    background: #181825;
    border-bottom: 1px solid #313244;
    flex-shrink: 0;
    overflow-x: auto;
    
    &::-webkit-scrollbar {
      height: 2px;
    }
    
    &::-webkit-scrollbar-thumb {
      background: #45475a;
    }
  }

  .tabs {
    display: flex;
    flex: 1;
    min-width: 0;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    background: transparent;
    border: none;
    color: #6c7086;
    font-size: 12px;
    cursor: pointer;
    white-space: nowrap;
    border-right: 1px solid #313244;
    transition: all 0.15s;

    &:hover {
      background: rgba(255, 255, 255, 0.05);
      color: #cdd6f4;
    }

    &.active {
      background: #1e1e2e;
      color: #cdd6f4;
    }

    .tab-status {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: #585b70;
      flex-shrink: 0;

      &.connected {
        background: #a6e3a1;
      }

      &.connecting {
        background: #f9e2af;
        animation: pulse 1s ease-in-out infinite;
      }
    }

    .tab-title {
      max-width: 120px;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .tab-close {
      width: 16px;
      height: 16px;
      padding: 0;
      border: none;
      background: transparent;
      color: inherit;
      cursor: pointer;
      border-radius: 3px;
      display: flex;
      align-items: center;
      justify-content: center;
      opacity: 0.6;
      font-size: 14px;

      &:hover {
        opacity: 1;
        background: rgba(255, 255, 255, 0.1);
      }
    }
  }

  .new-tab {
    width: 32px;
    height: 32px;
    margin: 4px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #6c7086;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
      color: #cdd6f4;
    }
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 12px;
    background: #313244;
    border-bottom: 1px solid #45475a;
    flex-shrink: 0;
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: #cdd6f4;

    .status-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: #585b70;
    }

    &.connected .status-dot {
      background: #a6e3a1;
    }

    &.connecting .status-dot {
      background: #f9e2af;
      animation: pulse 1s ease-in-out infinite;
    }
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
  }

  .font-size {
    font-size: 12px;
    color: #cdd6f4;
    min-width: 40px;
    text-align: center;
  }

  .divider {
    width: 1px;
    height: 16px;
    background: #45475a;
    margin: 0 4px;
  }

  button {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #cdd6f4;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.15s;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    &.danger:hover {
      background: rgba(243, 139, 168, 0.2);
      color: #f38ba8;
    }

    &.success:hover {
      background: rgba(166, 227, 161, 0.2);
      color: #a6e3a1;
    }

    &.active {
      background: rgba(137, 180, 250, 0.2);
      color: #89b4fa;
    }
  }

  .terminals-wrapper {
    flex: 1;
    position: relative;
    overflow: hidden;
  }

  .tab-content {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    overflow: hidden;

    &.hidden {
      visibility: hidden;
      pointer-events: none;
    }
  }

  .terminal-pane {
    width: 100%;
    height: 100%;
    padding: 4px;
    box-sizing: border-box;
    position: relative;

    &.active {
      box-shadow: inset 0 0 0 1px rgba(137, 180, 250, 0.3);
    }

    .pane-terminal {
      width: 100%;
      height: 100%;
    }

    :global(.xterm) {
      height: 100%;
    }

    :global(.xterm-viewport) {
      overflow-y: auto !important;
    }

    :global(.xterm-screen) {
      height: 100%;
    }

    /* 隐藏 xterm 辅助元素（用于键盘输入捕获） */
    :global(.xterm-helpers) {
      position: absolute !important;
      top: -9999px !important;
      left: -9999px !important;
      width: 1px !important;
      height: 1px !important;
      overflow: hidden !important;
      opacity: 0 !important;
      pointer-events: none !important;
    }

    :global(.xterm-helper-textarea) {
      position: absolute !important;
      top: 0 !important;
      left: 0 !important;
      width: 1px !important;
      height: 1px !important;
      opacity: 0 !important;
      pointer-events: none !important;
      z-index: -1 !important;
    }

    /* 修复选择文本颜色 - 覆盖 xterm 选择层 */
    :global(.xterm-selection) {
      opacity: 0.4 !important;
    }

    :global(.xterm-selection div) {
      opacity: 0.4 !important;
    }
  }

  .split-container {
    display: flex;
    width: 100%;
    height: 100%;

    &.horizontal {
      flex-direction: column;
    }

    &.vertical {
      flex-direction: row;
    }
  }

  .split-pane {
    overflow: hidden;
    min-width: 50px;
    min-height: 50px;
  }

  .split-resizer {
    flex-shrink: 0;
    background: #45475a;
    transition: background 0.15s ease;

    &.vertical {
      width: 4px;
      cursor: col-resize;

      &:hover, &.dragging {
        background: #89b4fa;
      }
    }

    &.horizontal {
      height: 4px;
      cursor: row-resize;

      &:hover, &.dragging {
        background: #89b4fa;
      }
    }
  }

  .search-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 12px;
    background: #313244;
    border-bottom: 1px solid #45475a;

    :global(.search-icon) {
      color: #6c7086;
      font-size: 16px;
    }

    input {
      flex: 1;
      background: #45475a;
      border: 1px solid transparent;
      border-radius: 4px;
      padding: 4px 8px;
      font-size: 12px;
      color: #cdd6f4;
      outline: none;

      &::placeholder {
        color: #6c7086;
      }

      &:focus {
        border-color: #89b4fa;
      }
    }

    button {
      width: 24px;
      height: 24px;
    }
  }

  .light {
    .search-bar {
      background: #e6e9ef;
      border-bottom-color: #ccd0da;

      input {
        background: #ffffff;
        color: #4c4f69;
        border-color: #ccd0da;

        &::placeholder {
          color: #9ca0b0;
        }

        &:focus {
          border-color: #1e66f5;
        }
      }

      :global(.search-icon) {
        color: #9ca0b0;
      }

      button {
        color: #4c4f69;

        &:hover {
          background: rgba(0, 0, 0, 0.1);
        }
      }
    }
  }

  .context-menu {
    position: absolute;
    z-index: 1000;
    min-width: 180px;
    background: #313244;
    border: 1px solid #45475a;
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);

    button {
      width: 100%;
      height: auto;
      padding: 8px 12px;
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
      text-align: left;
      border-radius: 4px;

      span {
        flex: 1;
      }

      .shortcut {
        flex: none;
        font-size: 11px;
        color: #6c7086;
      }

      &.danger {
        color: #f38ba8;
      }

      &.success {
        color: #a6e3a1;
      }
    }

    .menu-divider {
      height: 1px;
      background: #45475a;
      margin: 4px 8px;
    }
  }

  .light .context-menu {
    background: #ffffff;
    border-color: #ccd0da;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);

    button {
      color: #4c4f69;

      .shortcut {
        color: #9ca0b0;
      }

      &:hover {
        background: rgba(0, 0, 0, 0.05);
      }

      &.danger {
        color: #d20f39;
      }

      &.success {
        color: #40a02b;
      }
    }

    .menu-divider {
      background: #ccd0da;
    }
  }

  .quick-commands-panel {
    background: #313244;
    border-bottom: 1px solid #45475a;
    max-height: 300px;
    display: flex;
    flex-direction: column;
  }

  .quick-commands-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid #45475a;
    font-size: 12px;
    font-weight: 500;
    color: #cdd6f4;

    button {
      width: 20px;
      height: 20px;
      padding: 0;
    }
  }

  .quick-commands-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px;
  }

  .quick-command-item {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-bottom: 2px;

    .command-btn {
      flex: 1;
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 6px 10px;
      text-align: left;
      width: auto;
      height: auto;
      font-size: 12px;

      .command-name {
        flex-shrink: 0;
        font-weight: 500;
      }

      .command-text {
        flex: 1;
        color: #6c7086;
        font-family: monospace;
        font-size: 11px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .delete-btn {
      width: 24px;
      height: 24px;
      padding: 0;
      opacity: 0.5;
      flex-shrink: 0;

      &:hover {
        opacity: 1;
        color: #f38ba8;
      }
    }
  }

  .quick-commands-add {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px;
    border-top: 1px solid #45475a;

    input {
      background: #45475a;
      border: 1px solid transparent;
      border-radius: 4px;
      padding: 4px 8px;
      font-size: 12px;
      color: #cdd6f4;
      outline: none;

      &::placeholder {
        color: #6c7086;
      }

      &:focus {
        border-color: #89b4fa;
      }
    }

    .name-input {
      width: 80px;
    }

    .command-input {
      flex: 1;
    }

    button {
      width: 28px;
      height: 28px;

      &:disabled {
        opacity: 0.3;
        cursor: not-allowed;
      }
    }
  }

  .light {
    .quick-commands-panel {
      background: #e6e9ef;
      border-bottom-color: #ccd0da;
    }

    .quick-commands-header {
      color: #4c4f69;
      border-bottom-color: #ccd0da;
    }

    .quick-command-item {
      .command-btn {
        color: #4c4f69;

        .command-text {
          color: #9ca0b0;
        }

        &:hover {
          background: rgba(0, 0, 0, 0.05);
        }
      }

      .delete-btn:hover {
        color: #d20f39;
      }
    }

    .quick-commands-add {
      border-top-color: #ccd0da;

      input {
        background: #ffffff;
        color: #4c4f69;

        &::placeholder {
          color: #9ca0b0;
        }

        &:focus {
          border-color: #1e66f5;
        }
      }

      button {
        color: #4c4f69;
      }
    }

    .split-resizer {
      background: #ccd0da;

      &.vertical, &.horizontal {
        &:hover, &.dragging {
          background: #1e66f5;
        }
      }
    }

    .terminal-pane.active {
      box-shadow: inset 0 0 0 1px rgba(30, 102, 245, 0.3);
    }
  }

  /* SFTP Browser Overlay */
  .sftp-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 3000;
    display: flex;
  }

  .sftp-btn {
    color: #a6e3a1 !important;

    &:hover {
      background: rgba(166, 227, 161, 0.1) !important;
    }
  }

  /* SSH Panel & Modal Styles */
  .new-ssh {
    background: transparent;
    border: none;
    color: #a6adc8;
    cursor: pointer;
    padding: 4px 8px;
    transition: all 0.15s;

    &:hover, &.active {
      color: #89b4fa;
    }
  }

  :global(.tab-icon) {
    font-size: 12px;
    margin-right: 4px;
    opacity: 0.6;
  }

  :global(.tab-icon.ssh) {
    color: #a6e3a1;
  }

  .ssh-panel {
    position: absolute;
    top: calc(var(--tab-bar-height, 36px) + var(--toolbar-height, 36px));
    right: 8px;
    width: 320px;
    max-height: 400px;
    background: #1e1e2e;
    border: 1px solid #45475a;
    border-radius: 8px;
    z-index: 1000;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.3);
  }

  .ssh-panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    border-bottom: 1px solid #45475a;
    font-size: 13px;
    font-weight: 500;
    color: #cdd6f4;

    .ssh-panel-actions {
      display: flex;
      gap: 4px;

      button {
        width: 24px;
        height: 24px;
        padding: 0;
        background: transparent;
        border: none;
        color: #a6adc8;
        cursor: pointer;
        border-radius: 4px;

        &:hover {
          color: #cdd6f4;
          background: #45475a;
        }
      }
    }
  }

  .ssh-list {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
  }

  .ssh-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 24px;
    color: #6c7086;

    .add-btn {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 8px 16px;
      background: #89b4fa;
      color: #1e1e2e;
      border: none;
      border-radius: 6px;
      cursor: pointer;
      font-size: 12px;
      white-space: nowrap;

      &:hover {
        background: #74a8fc;
      }
    }
  }

  .ssh-item {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px;
    margin-bottom: 4px;
    border-radius: 6px;

    &:hover {
      background: rgba(255, 255, 255, 0.03);
    }
  }

  .ssh-connect-btn {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 12px;
    background: transparent;
    border: none;
    color: #cdd6f4;
    cursor: pointer;
    border-radius: 4px;
    text-align: left;

    &:hover {
      background: rgba(137, 180, 250, 0.1);
    }
  }

  .ssh-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .ssh-name {
    font-size: 13px;
    font-weight: 500;
  }

  .ssh-host {
    font-size: 11px;
    color: #6c7086;
    font-family: monospace;
  }

  .ssh-item-actions {
    display: flex;
    gap: 2px;
    opacity: 0;
    transition: opacity 0.15s;

    button {
      width: 28px;
      height: 28px;
      padding: 0;
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      border-radius: 4px;

      &:hover {
        color: #cdd6f4;
        background: #45475a;
      }

      &.danger:hover {
        color: #f38ba8;
        background: rgba(243, 139, 168, 0.1);
      }
    }
  }

  .ssh-item:hover .ssh-item-actions {
    opacity: 1;
  }

  /* SSH Modal */
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 2000;
  }

  .ssh-modal {
    width: 420px;
    max-height: 90vh;
    background: #1e1e2e;
    border: 1px solid #45475a;
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    overflow: hidden;

    &.shake {
      animation: shake 0.5s ease-in-out;
    }
  }

  @keyframes shake {
    0%, 100% { transform: translateX(0); }
    10%, 30%, 50%, 70%, 90% { transform: translateX(-8px); }
    20%, 40%, 60%, 80% { transform: translateX(8px); }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid #45475a;

    h3 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
      color: #cdd6f4;
    }

    button {
      width: 28px;
      height: 28px;
      padding: 0;
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      border-radius: 4px;

      &:hover {
        color: #cdd6f4;
        background: #45475a;
      }
    }
  }

  .modal-body {
    flex: 1;
    padding: 20px;
    overflow-y: auto;
  }

  .form-group {
    margin-bottom: 16px;

    label {
      display: block;
      margin-bottom: 6px;
      font-size: 12px;
      color: #a6adc8;
    }

    input, textarea {
      width: 100%;
      padding: 10px 12px;
      font-size: 13px;
      background: #313244;
      border: 1px solid #45475a;
      border-radius: 6px;
      color: #cdd6f4;
      outline: none;
      box-sizing: border-box;

      &::placeholder {
        color: #6c7086;
      }

      &:focus {
        border-color: #89b4fa;
      }
    }

    textarea {
      resize: vertical;
      font-family: monospace;
      font-size: 12px;
    }
  }

  .form-row {
    display: flex;
    gap: 12px;

    .flex-1 {
      flex: 1;
    }

    .port {
      width: 100px;
    }
  }

  .auth-type-tabs {
    display: flex;
    background: #313244;
    border-radius: 6px;
    padding: 2px;

    button {
      flex: 1;
      padding: 8px 12px;
      font-size: 12px;
      background: transparent;
      border: none;
      color: #a6adc8;
      cursor: pointer;
      border-radius: 4px;
      transition: all 0.15s;

      &.active {
        background: #89b4fa;
        color: #1e1e2e;
      }

      &:not(.active):hover {
        color: #cdd6f4;
      }
    }
  }

  .form-error {
    padding: 10px 12px;
    background: rgba(243, 139, 168, 0.1);
    border: 1px solid #f38ba8;
    border-radius: 6px;
    color: #f38ba8;
    font-size: 12px;

    &.success {
      background: rgba(166, 227, 161, 0.1);
      border-color: #a6e3a1;
      color: #a6e3a1;
    }
  }

  .modal-footer {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 16px 20px;
    border-top: 1px solid #45475a;

    .flex-1 {
      flex: 1;
    }

    button {
      padding: 10px 16px;
      font-size: 13px;
      border-radius: 6px;
      cursor: pointer;
      transition: all 0.15s;
      white-space: nowrap;

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
    }

    .btn-primary {
      background: #89b4fa;
      color: #1e1e2e;
      border: none;
      min-width: fit-content;

      &:hover:not(:disabled) {
        background: #74a8fc;
      }
    }

    .btn-secondary {
      background: transparent;
      color: #a6adc8;
      border: 1px solid #45475a;
      min-width: fit-content;

      &:hover:not(:disabled) {
        color: #cdd6f4;
        border-color: #6c7086;
      }
    }
  }

  /* Light theme overrides for SSH */
  .light {
    .new-ssh {
      color: #6c6f85;

      &:hover, &.active {
        color: #1e66f5;
      }
    }

    .ssh-panel {
      background: #eff1f5;
      border-color: #ccd0da;
    }

    .ssh-panel-header {
      color: #4c4f69;
      border-bottom-color: #ccd0da;

      .ssh-panel-actions button {
        color: #6c6f85;

        &:hover {
          color: #4c4f69;
          background: #e6e9ef;
        }
      }
    }

    .ssh-empty {
      color: #6c6f85;

      .add-btn {
        background: #1e66f5;
        color: white;
        min-width: fit-content;

        &:hover {
          background: #1a5ae0;
        }
      }
    }

    .ssh-item:hover {
      background: rgba(0, 0, 0, 0.03);
    }

    .ssh-connect-btn {
      color: #4c4f69;

      &:hover {
        background: rgba(30, 102, 245, 0.1);
      }
    }

    .ssh-host {
      color: #9ca0b0;
    }

    .ssh-item-actions button {
      color: #6c6f85;

      &:hover {
        color: #4c4f69;
        background: #e6e9ef;
      }

      &.danger:hover {
        color: #d20f39;
        background: rgba(210, 15, 57, 0.1);
      }
    }

    .ssh-modal {
      background: #eff1f5;
      border-color: #ccd0da;
    }

    .modal-header {
      border-bottom-color: #ccd0da;

      h3 {
        color: #4c4f69;
      }

      button {
        color: #6c6f85;

        &:hover {
          color: #4c4f69;
          background: #e6e9ef;
        }
      }
    }

    .form-group {
      label {
        color: #6c6f85;
      }

      input, textarea {
        background: #ffffff;
        border-color: #ccd0da;
        color: #4c4f69;

        &::placeholder {
          color: #9ca0b0;
        }

        &:focus {
          border-color: #1e66f5;
        }
      }
    }

    .auth-type-tabs {
      background: #e6e9ef;

      button {
        color: #6c6f85;

        &.active {
          background: #1e66f5;
          color: white;
        }

        &:not(.active):hover {
          color: #4c4f69;
        }
      }
    }

    .form-error {
      background: rgba(210, 15, 57, 0.1);
      border-color: #d20f39;
      color: #d20f39;

      &.success {
        background: rgba(64, 160, 43, 0.1);
        border-color: #40a02b;
        color: #40a02b;
      }
    }

    .modal-footer {
      border-top-color: #ccd0da;

      .btn-primary {
        background: #1e66f5;

        &:hover:not(:disabled) {
          background: #1a5ae0;
        }
      }

      .btn-secondary {
        color: #6c6f85;
        border-color: #ccd0da;

        &:hover:not(:disabled) {
          color: #4c4f69;
          border-color: #9ca0b0;
        }
      }
    }
  }
</style>
