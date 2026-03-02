<script lang="ts">
  import Icon from "@iconify/svelte";
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import { theme } from "$shared/stores/theme.svelte";
  import { settings } from "$shared/stores/settings.svelte";
  import { userStore } from "$shared/stores/user.svelte";
  import { wallpaper, type WallpaperItem, type WallpaperType } from "$desktop/stores/wallpaper.svelte";
  import { toast } from "$shared/stores/toast.svelte";
  import { apps } from "$desktop/stores/apps.svelte";
  import { desktop } from "$desktop/stores/desktop.svelte";
  import { remoteAccessStore } from "$desktop/stores/remote-access.svelte";
  import {
    notificationBubbleStore,
    type BubbleNotificationMode,
  } from "$shared/stores/notification-bubble.svelte";
  import {
    Switch,
    Progress,
    Button,
    Input,
    Select,
    Modal,
    Alert,
    Spinner,
  } from "$shared/ui";
  import { setupApi, type FactoryResetRequest } from "$shared/services/setup";
  import { systemService } from "$shared/services/system";
  import { api } from "$shared/services/api";
  import { authService } from "$shared/services/auth";
  import { usersService, type User as ManagedUser, type CreateUserRequest } from "$shared/services/users";
  import { getAvatarUrl } from "$shared/utils/avatar";
  import {
    notificationService,
    channelTypeInfo,
    allChannelTypes,
    type NotificationChannel,
    type ChannelType,
  } from "$shared/services/notification";
  import LanguageSettings from "./LanguageSettings.svelte";
  import { i18nStore, currentLanguage } from "$lib/i18n/store";
  import { getI18nOptions, type I18nOptionsResponse } from "$lib/i18n/api";

  interface SettingSection {
    id: string;
    name: string;
    icon: string;
  }

  const sections = $derived<SettingSection[]>([
    { id: "account", name: $t("settingsPage.sidebar.account"), icon: "mdi:account-circle" },
    { id: "storage", name: $t("settingsPage.sidebar.storage"), icon: "mdi:harddisk" },
    { id: "network", name: $t("settingsPage.sidebar.network"), icon: "mdi:lan" },
    { id: "appearance", name: $t("settingsPage.sidebar.appearance"), icon: "mdi:palette" },
    { id: "notification", name: $t("settingsPage.sidebar.notification"), icon: "mdi:bell" },
    { id: "security", name: $t("settingsPage.sidebar.security"), icon: "mdi:shield-lock" },
    { id: "about", name: $t("settingsPage.sidebar.about"), icon: "mdi:information" },
  ]);

  // 接收从窗口传递的 section 参数
  let { section }: { section?: string } = $props();

  // 仅取 prop 初始值，之后由用户点击切换
  // svelte-ignore state_referenced_locally
  let activeSection = $state(section || "account");

  /* 账户设置 - 从当前登录用户加载 */
  let accountUser = $state<ManagedUser | null>(null);
  let showPasswordModal = $state(false);
  let currentPassword = $state("");
  let newPassword = $state("");
  let confirmPassword = $state("");
  let avatarInput = $state<HTMLInputElement | null>(null);
  let avatarUploading = $state(false);

  /* 中国源加速（内联） */
  let cnMirrorOptions = $state<I18nOptionsResponse | null>(null);
  let cnMirrorEnabled = $state(false);
  let cnMirrors = $state<Record<string, string>>({});

  // 从 store 同步镜像状态
  $effect(() => {
    const storeMirrors = $i18nStore.mirrors;
    if (storeMirrors) {
      cnMirrors = { ...storeMirrors };
      cnMirrorEnabled = Object.values(storeMirrors).some(v => v && v !== "follow");
    }
  });

  // 中国源加速 — 内联逻辑
  async function loadCnMirrorOptions() {
    try {
      cnMirrorOptions = await getI18nOptions();
    } catch (e) {
      console.error("Failed to load mirror options:", e);
    }
  }

  function getCnMirrorServices() {
    return (cnMirrorOptions?.services || []).filter(
      (s) => (cnMirrorOptions?.mirrors?.cn?.[s.id]?.length ?? 0) > 0
    );
  }

  function getCnMirrorSelectOptions(serviceId: string) {
    const result: { value: string; label: string }[] = [
      { value: "", label: "官方源" },
    ];
    const cnList = cnMirrorOptions?.mirrors?.cn?.[serviceId];
    if (cnList) {
      for (const m of cnList) {
        if (m.url) result.push({ value: m.url, label: m.name });
      }
    }
    return result;
  }

  function getCnMirrorValue(serviceId: string): string {
    const v = cnMirrors[serviceId];
    return (!v || v === "follow") ? "" : v;
  }

  function handleCnMirrorToggle(checked: boolean) {
    cnMirrorEnabled = checked;
    if (checked && cnMirrorOptions?.services) {
      const newMirrors = { ...cnMirrors };
      for (const service of cnMirrorOptions.services) {
        if (!newMirrors[service.id] || newMirrors[service.id] === "follow") {
          const cnList = cnMirrorOptions.mirrors?.cn?.[service.id];
          if (cnList && cnList.length > 0 && cnList[0].url) {
            newMirrors[service.id] = cnList[0].url;
          }
        }
      }
      cnMirrors = newMirrors;
      saveCnMirrors(newMirrors);
    } else if (!checked) {
      cnMirrors = {};
      saveCnMirrors({});
    }
  }

  function handleCnMirrorChange(serviceId: string, value: string) {
    if (!value) {
      const m = { ...cnMirrors };
      delete m[serviceId];
      cnMirrors = m;
    } else {
      cnMirrors = { ...cnMirrors, [serviceId]: value };
    }
    saveCnMirrors(cnMirrors);
  }

  function saveCnMirrors(m: Record<string, string>) {
    const toSave: Record<string, string> = {};
    for (const [key, value] of Object.entries(m)) {
      toSave[key] = value || "follow";
    }
    const hasAny = Object.values(toSave).some(v => v && v !== "follow");
    i18nStore.updateSettings({
      region: hasAny ? "cn" : "intl",
      mirrors: toSave,
    });
    toast.success("镜像源设置已更新");
  }

  function getCnServiceName(name: Record<string, string>): string {
    return name["zh-CN"] || name["en-US"] || "";
  }

  // 加载当前用户信息（直接调 /users/current，不依赖 userStore）
  async function loadCurrentUser() {
    try {
      const resp = await api.get<{ data: ManagedUser }>("/users/current");
      if (resp.data) {
        accountUser = resp.data;
        // 同步到 userStore
        userStore.updateUser({
          id: resp.data.id,
          username: resp.data.username,
          role: resp.data.role as "admin" | "user",
        });
      }
    } catch (e) {
      console.error($t("settingsPage.account.loadUserFailed"), e);
    }
  }

  async function handleAvatarUpload(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file || !accountUser) return;
    if (file.size > 2 * 1024 * 1024) {
      alert($t("settingsPage.account.avatarTooLarge"));
      return;
    }
    avatarUploading = true;
    try {
      const result = await usersService.uploadAvatar(accountUser.id, file);
      if (result.success && result.avatar) {
        const newAvatar = result.avatar + "?t=" + Date.now();
        accountUser = { ...accountUser, avatar: newAvatar };
        // 同步到 userStore，让开始菜单的头像也更新
        userStore.updateUser({ avatar: newAvatar });
      } else {
        alert(result.message || $t("settingsPage.account.uploadFailed"));
      }
    } finally {
      avatarUploading = false;
      input.value = "";
    }
  }

  /* 用户管理 */
  let managedUsers = $state<ManagedUser[]>([]);
  let usersLoading = $state(false);
  let showCreateUserModal = $state(false);
  let showDeleteUserConfirm = $state(false);
  let showResetPasswordModal = $state(false);
  let newUserUsername = $state("");
  let newUserPassword = $state("");
  let newUserNickname = $state("");
  let deleteTargetUser = $state<ManagedUser | null>(null);
  let resetPasswordTargetUser = $state<ManagedUser | null>(null);
  let resetNewPassword = $state("");
  let creatingUser = $state(false);

  async function loadUsers() {
    usersLoading = true;
    try {
      managedUsers = await usersService.list();
    } catch {
      toast.error($t("settingsPage.users.loadFailed"));
    } finally {
      usersLoading = false;
    }
  }

  async function handleCreateUser() {
    if (!newUserUsername || !newUserPassword) {
      toast.error($t("settingsPage.users.usernamePasswordRequired"));
      return;
    }
    if (newUserUsername.length < 3) {
      toast.error($t("settingsPage.users.usernameMinLength"));
      return;
    }
    if (newUserPassword.length < 6) {
      toast.error($t("settingsPage.users.passwordMinLength"));
      return;
    }
    creatingUser = true;
    try {
      const result = await usersService.create({
        username: newUserUsername,
        password: newUserPassword,
        nickname: newUserNickname || newUserUsername,
      });
      if (result.success) {
        toast.success($t("settingsPage.users.createSuccess", { values: { username: newUserUsername } }));
        showCreateUserModal = false;
        newUserUsername = "";
        newUserPassword = "";
        newUserNickname = "";
        await loadUsers();
      } else {
        toast.error(result.message || $t("settingsPage.users.createFailed"));
      }
    } catch {
      toast.error($t("settingsPage.users.createUserFailed"));
    } finally {
      creatingUser = false;
    }
  }

  function confirmDeleteUser(u: ManagedUser) {
    deleteTargetUser = u;
    showDeleteUserConfirm = true;
  }

  async function handleDeleteUser() {
    if (!deleteTargetUser) return;
    const result = await usersService.delete(deleteTargetUser.id);
    if (result.success) {
      toast.success($t("settingsPage.users.userDeleted", { values: { username: deleteTargetUser.username } }));
      showDeleteUserConfirm = false;
      deleteTargetUser = null;
      await loadUsers();
    } else {
      toast.error(result.message || $t("settingsPage.users.deleteFailed"));
    }
  }

  function openResetPassword(u: ManagedUser) {
    resetPasswordTargetUser = u;
    resetNewPassword = "";
    showResetPasswordModal = true;
  }

  async function handleResetPassword() {
    if (!resetPasswordTargetUser || !resetNewPassword) return;
    if (resetNewPassword.length < 6) {
      toast.error($t("settingsPage.users.newPasswordMinLength"));
      return;
    }
    const result = await usersService.resetPassword(resetPasswordTargetUser.id, resetNewPassword);
    if (result.success) {
      toast.success($t("settingsPage.users.passwordReset"));
      showResetPasswordModal = false;
    } else {
      toast.error(result.message || $t("settingsPage.users.resetFailed"));
    }
  }

  function formatDate(dateStr?: string): string {
    if (!dateStr) return $t("settingsPage.users.never");
    try {
      const d = new Date(dateStr);
      return d.toLocaleString("zh-CN");
    } catch {
      return dateStr;
    }
  }

  /* 网络设置 */
  let hostname = $state("RDE-Server");
  let dhcpEnabled = $state(true);
  let ipAddress = $state("192.168.1.100");
  let gateway = $state("192.168.1.1");
  let dns = $state("8.8.8.8");

  /* 代理设置 */
  type ProxyMode = "off" | "manual" | "pac";
  let proxyMode = $state<ProxyMode>("off");
  let httpProxy = $state("");
  let httpsProxy = $state("");
  let socks5Proxy = $state("");
  let noProxy = $state("localhost,127.0.0.1,*.local");
  let pacUrl = $state("");
  let proxyAuthEnabled = $state(false);
  let proxyUsername = $state("");
  let proxyPassword = $state("");
  let dockerMirror = $state("");
  let dockerProxyEnabled = $state(false);
  let proxyTesting = $state(false);
  let proxyTestResult = $state<{ success: boolean; message: string } | null>(null);

  // 保存代理设置
  async function saveProxySettings() {
    try {
      const config = {
        mode: proxyMode,
        http_proxy: httpProxy,
        https_proxy: httpsProxy,
        socks5: socks5Proxy,
        no_proxy: noProxy,
        pac_url: pacUrl,
        auth: proxyAuthEnabled ? { username: proxyUsername, password: proxyPassword } : null,
        docker_mirror: dockerMirror,
        docker_proxy_enabled: dockerProxyEnabled,
      };
      const result = await systemService.saveProxyConfig(config);
      if (result.success) {
        toast.success($t("settingsPage.network.proxySaved"));
      } else {
        toast.error($t("settingsPage.network.saveFailed"));
      }
    } catch {
      toast.error($t("settingsPage.network.saveProxyFailed"));
    }
  }

  // 测试代理连接
  async function testProxyConnection() {
    proxyTesting = true;
    proxyTestResult = null;
    try {
      const result = await systemService.testProxy({
        proxy_url: httpProxy || httpsProxy || socks5Proxy,
        test_url: "https://www.google.com",
      });
      proxyTestResult = result.success
        ? { success: true, message: $t("settingsPage.network.proxySuccess") }
        : { success: false, message: result.message || $t("settingsPage.network.connectionFailed") };
    } catch {
      proxyTestResult = { success: false, message: $t("settingsPage.network.testFailed") };
    } finally {
      proxyTesting = false;
    }
  }

  // 加载代理配置
  async function loadProxyConfig() {
    try {
      const result = await systemService.getProxyConfig();
      if (result.success && result.data) {
        proxyMode = result.data.mode || "off";
        httpProxy = result.data.http_proxy || "";
        httpsProxy = result.data.https_proxy || "";
        socks5Proxy = result.data.socks5 || "";
        noProxy = result.data.no_proxy || "localhost,127.0.0.1,*.local";
        pacUrl = result.data.pac_url || "";
        dockerMirror = result.data.docker_mirror || "";
        dockerProxyEnabled = result.data.docker_proxy_enabled || false;
        if (result.data.auth) {
          proxyAuthEnabled = true;
          proxyUsername = result.data.auth.username || "";
          proxyPassword = "";
        }
      }
    } catch {
      // 忽略加载错误
    }
  }

  /* 存储信息 - 使用 $derived 以响应语言变化 */
  let storageInfo = $derived([
    { name: $t("settingsPage.storage.systemDisk"), path: "/dev/sda1", total: 256, used: 45, type: "SSD" },
    { name: $t("settingsPage.storage.dataDisk") + " 1", path: "/dev/sdb1", total: 2048, used: 1280, type: "HDD" },
    { name: $t("settingsPage.storage.dataDisk") + " 2", path: "/dev/sdc1", total: 4096, used: 2560, type: "HDD" },
  ]);

  /* 外观设置 */
  let currentThemeMode = $derived(theme.mode);
  let accentColor = $derived(theme.accentColor);

  // 壁纸分类标签
  let wallpaperTab = $state<"static" | "webgl" | "lottie">("static");

  // 自定义壁纸弹窗
  let showCustomWallpaperModal = $state(false);
  let customWallpaperUrl = $state("");
  let customWallpaperFile = $state<File | null>(null);

  // 选择壁纸
  function selectWallpaper(item: WallpaperItem) {
    wallpaper.setWallpaper(item.id, item.type);
  }

  // 打开自定义壁纸弹窗
  function openCustomWallpaperModal() {
    showCustomWallpaperModal = true;
    customWallpaperUrl = "";
    customWallpaperFile = null;
  }

  // 处理自定义壁纸文件选择
  function handleCustomWallpaperFile(e: Event) {
    const input = e.target as HTMLInputElement;
    if (input.files && input.files[0]) {
      customWallpaperFile = input.files[0];
    }
  }

  // 应用自定义壁纸
  async function applyCustomWallpaper() {
    if (customWallpaperFile) {
      // 转换为 base64 或上传到服务器
      const reader = new FileReader();
      reader.onload = (e) => {
        const dataUrl = e.target?.result as string;
        wallpaper.setCustomWallpaper(dataUrl);
        showCustomWallpaperModal = false;
      };
      reader.readAsDataURL(customWallpaperFile);
    } else if (customWallpaperUrl) {
      wallpaper.setCustomWallpaper(customWallpaperUrl);
      showCustomWallpaperModal = false;
    }
  }

  onMount(async () => {
    await wallpaper.loadIndex();
    // 加载当前用户信息
    await loadCurrentUser();
    // 加载中国源选项
    loadCnMirrorOptions();
    // 加载 2FA 状态
    twoFactorEnabled = await authService.get2FAStatus();
    // 加载远程访问设置
    await loadRemoteAccessSettings();
    // 加载通知渠道
    await loadNotificationChannels();
    // 加载代理配置
    await loadProxyConfig();
    // 加载用户列表
    await loadUsers();
    // 加载版本号
    try {
      const info = await systemService.getInfo();
      if (info.data?.version) {
        currentVersion = info.data.version;
      }
    } catch {
      // 忽略
    }
  });

  /* 通知设置 */
  let desktopNotification = $state(true);
  let soundEnabled = $state(true);
  let emailNotification = $state(false);

  /* 安全设置 - 2FA */
  let twoFactorEnabled = $state(false);
  let twoFactorLoading = $state(false);
  let show2FASetupModal = $state(false);
  let twoFASetupData = $state<{
    secret?: string;
    qr_code_url?: string;
    backup_codes?: string[];
  } | null>(null);
  let twoFAVerifyCode = $state("");
  let twoFAStep = $state<"qr" | "verify" | "done">("qr");
  let backupCodesCopied = $state(false);

  /* 远程访问设置 */
  interface RemoteAccessSettings {
    ssh_enabled: boolean;
    ssh_running: boolean;
    ssh_port: number;
    terminal_enabled: boolean;
  }
  let remoteAccessSettings = $state<RemoteAccessSettings | null>(null);
  let remoteAccessLoading = $state(true);
  let remoteAccessError = $state("");

  /* 推送渠道设置 */
  let notificationChannels = $state<NotificationChannel[]>([]);
  let channelsLoading = $state(false);
  let showChannelModal = $state(false);
  let editingChannel = $state<NotificationChannel | null>(null);
  let channelForm = $state<{
    name: string;
    type: ChannelType;
    config: Record<string, string>;
    description: string;
  }>({
    name: "",
    type: "email",
    config: {},
    description: "",
  });

  async function loadNotificationChannels() {
    channelsLoading = true;
    try {
      const response = await notificationService.getChannels();
      if (response.success && response.data) {
        notificationChannels = response.data;
      }
    } catch (e) {
      console.error("加载推送渠道失败", e);
    } finally {
      channelsLoading = false;
    }
  }

  function openAddChannelModal() {
    editingChannel = null;
    channelForm = { name: "", type: "email", config: {}, description: "" };
    showChannelModal = true;
  }

  function openEditChannelModal(channel: NotificationChannel) {
    editingChannel = channel;
    channelForm = {
      name: channel.name,
      type: channel.type,
      config: JSON.parse(channel.config || "{}"),
      description: channel.description,
    };
    showChannelModal = true;
  }

  async function saveChannel() {
    try {
      if (editingChannel) {
        await notificationService.updateChannel(editingChannel.id, {
          name: channelForm.name,
          config: channelForm.config,
          description: channelForm.description,
        });
        toast.success($t("settingsPage.notification.channelUpdated"));
      } else {
        await notificationService.createChannel({
          name: channelForm.name,
          type: channelForm.type,
          config: channelForm.config,
          description: channelForm.description,
        });
        toast.success($t("settingsPage.notification.channelCreated"));
      }
      showChannelModal = false;
      await loadNotificationChannels();
    } catch (e) {
      toast.error($t("settingsPage.notification.saveFailed"));
    }
  }

  async function toggleChannelEnabled(channel: NotificationChannel) {
    await notificationService.updateChannel(channel.id, { enabled: !channel.enabled });
    await loadNotificationChannels();
  }

  async function testChannel(channel: NotificationChannel) {
    const result = await notificationService.testChannel(channel.id);
    if (result.success) {
      toast.success($t("settingsPage.notification.testSent"));
    } else {
      toast.error($t("settingsPage.notification.testFailed") + ": " + result.message);
    }
  }

  async function deleteChannel(channel: NotificationChannel) {
    if (!confirm($t("settingsPage.notification.deleteChannelConfirm", { values: { name: channel.name } }))) return;
    await notificationService.deleteChannel(channel.id);
    await loadNotificationChannels();
    toast.success($t("settingsPage.notification.channelDeleted"));
  }

  // 密码确认弹窗
  let showSecurityConfirmModal = $state(false);
  let securityConfirmPassword = $state("");
  let securityConfirmError = $state("");
  let securityConfirmLoading = $state(false);
  let pendingSecurityAction = $state<{
    type: "ssh" | "terminal";
    enabled: boolean;
  } | null>(null);

  async function loadRemoteAccessSettings() {
    remoteAccessLoading = true;
    remoteAccessError = "";
    try {
      const response = await systemService.getRemoteAccessSettings();
      if (response.success === 200 && response.data) {
        remoteAccessSettings = response.data;
      } else {
        remoteAccessError = $t("settingsPage.security.loadRemoteAccessFailed");
      }
    } catch (e) {
      remoteAccessError = $t("settingsPage.security.loadRemoteAccessFailed");
      console.error(e);
    } finally {
      remoteAccessLoading = false;
    }
  }

  // 打开安全确认弹窗
  function openSecurityConfirm(type: "ssh" | "terminal", enabled: boolean) {
    pendingSecurityAction = { type, enabled };
    securityConfirmPassword = "";
    securityConfirmError = "";
    showSecurityConfirmModal = true;
  }

  // 执行安全设置更新
  async function executeSecurityUpdate() {
    if (!securityConfirmPassword) {
      securityConfirmError = $t("settingsPage.security.confirmPassword");
      return;
    }
    if (!pendingSecurityAction) return;

    securityConfirmLoading = true;
    securityConfirmError = "";

    try {
      const updateData: { password: string; ssh_enabled?: boolean; terminal_enabled?: boolean } = {
        password: securityConfirmPassword,
      };

      if (pendingSecurityAction.type === "ssh") {
        updateData.ssh_enabled = pendingSecurityAction.enabled;
      } else {
        updateData.terminal_enabled = pendingSecurityAction.enabled;
      }

      const response = await systemService.updateRemoteAccessSettings(updateData);
      if (response.success === 200) {
        await loadRemoteAccessSettings();
        // 同步更新全局 store，以便桌面图标、开始菜单、右键菜单立即更新
        await remoteAccessStore.load();

        // 启用终端时，自动在桌面添加终端图标；禁用时移除
        if (pendingSecurityAction.type === "terminal") {
          if (pendingSecurityAction.enabled) {
            desktop.addIcon({ name: $t("terminal.tabTitle") || "终端", icon: "/icons/terminal.svg", appId: "terminal", x: 0, y: 0 });
          } else {
            desktop.removeIconByAppId("terminal");
          }
        }

        const actionName = pendingSecurityAction.type === "ssh" ? $t("settingsPage.security.sshService") : $t("settingsPage.security.webTerminal");
        const statusText = pendingSecurityAction.enabled ? $t("settingsPage.notification.enabled") : $t("settingsPage.notification.disabled");
        toast.success(`${actionName}${statusText}`);
        showSecurityConfirmModal = false;
      } else {
        securityConfirmError = response.message || $t("settingsPage.security.actionFailed");
      }
    } catch (e: any) {
      securityConfirmError = e.message || $t("settingsPage.security.actionFailed");
      console.error(e);
    } finally {
      securityConfirmLoading = false;
    }
  }

  /* 电源操作 */
  let showPowerConfirmModal = $state(false);
  let powerAction = $state<"restart" | "shutdown">("restart");
  let powerLoading = $state(false);

  function confirmPower(action: "restart" | "shutdown") {
    powerAction = action;
    showPowerConfirmModal = true;
  }

  async function executePowerAction() {
    powerLoading = true;
    try {
      if (powerAction === "restart") {
        await systemService.reboot();
        toast.success($t("settingsPage.security.restartSuccess"));
      } else {
        await systemService.shutdown();
        toast.success($t("settingsPage.security.shutdownSuccess"));
      }
    } catch (e: any) {
      toast.error(e.message || $t("settingsPage.security.powerError"));
    } finally {
      powerLoading = false;
      showPowerConfirmModal = false;
    }
  }

  /* 恢复出厂设置 */
  let showFactoryResetModal = $state(false);
  let factoryResetStep = $state<"warning" | "confirm" | "processing">("warning");
  let factoryResetPassword = $state("");
  let factoryResetConfirmText = $state("");
  let factoryResetKeepDocker = $state(false);
  let factoryResetKeepFiles = $state(false);
  let factoryResetError = $state("");
  let factoryResetProgress = $state<string[]>([]);

  function openFactoryResetModal() {
    showFactoryResetModal = true;
    factoryResetStep = "warning";
    factoryResetPassword = "";
    factoryResetConfirmText = "";
    factoryResetKeepDocker = false;
    factoryResetKeepFiles = false;
    factoryResetError = "";
    factoryResetProgress = [];
  }

  function closeFactoryResetModal() {
    showFactoryResetModal = false;
  }

  async function executeFactoryReset() {
    if (!factoryResetPassword) {
      factoryResetError = $t("settingsPage.security.confirmIdentity");
      return;
    }

    if (factoryResetConfirmText !== "RESET") {
      factoryResetError = $t("settingsPage.security.confirmReset");
      return;
    }

    factoryResetStep = "processing";
    factoryResetError = "";
    factoryResetProgress = [$t("settingsPage.security.verifying")];

    try {
      const request: FactoryResetRequest = {
        password: factoryResetPassword,
        confirm_text: factoryResetConfirmText,
        keep_docker_apps: factoryResetKeepDocker,
        keep_user_files: factoryResetKeepFiles,
      };

      factoryResetProgress = [...factoryResetProgress, $t("settingsPage.security.clearingData")];
      const response = await setupApi.factoryReset(request);
      factoryResetProgress = [...factoryResetProgress, $t("settingsPage.security.resetComplete")];

      setTimeout(() => {
        window.location.href = response.redirect_url || "/setup";
      }, 2000);
    } catch (e) {
      factoryResetError = e instanceof Error ? e.message : $t("settingsPage.security.resetFailed");
      factoryResetStep = "confirm";
    }
  }

  /* 电源设置 */
  let sleepTime = $state("30");

  /* 版本号 */
  let currentVersion = $state("0.0.0");

  /* 系统信息 */
  let systemInfo = {
    deviceName: "RDE-Server",
    uptime: "3 天 12 小时 34 分钟",
    cpu: "Intel Core i5-12400",
    memory: "16 GB DDR4",
    kernel: "Linux 6.1.0",
    arch: "x86_64",
  };

  function setTheme(mode: "light" | "dark" | "auto") {
    theme.set(mode);
  }

  function formatSize(gb: number): string {
    if (gb >= 1024) {
      return `${(gb / 1024).toFixed(1)} TB`;
    }
    return `${gb} GB`;
  }

  function handlePasswordChange() {
    if (!accountUser) return;
    if (newPassword.length < 6) {
      toast.error($t("settingsPage.password.minLength"));
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error($t("settingsPage.password.mismatch"));
      return;
    }
    usersService.changePassword(accountUser.id, {
      old_password: currentPassword,
      new_password: newPassword,
    }).then((result) => {
      if (result.success) {
        toast.success($t("settingsPage.password.changeSuccess"));
        showPasswordModal = false;
        currentPassword = "";
        newPassword = "";
        confirmPassword = "";
      } else {
        toast.error(result.message || $t("settingsPage.password.changeFailed"));
      }
    });
  }

  // ----- 两步验证 -----
  async function handleSetup2FA() {
    twoFactorLoading = true;
    const result = await authService.setup2FA();
    twoFactorLoading = false;
    if (result.success) {
      twoFASetupData = result;
      twoFAStep = "qr";
      twoFAVerifyCode = "";
      show2FASetupModal = true;
    } else {
      toast.error(result.message || $t("settingsPage.twoFactor.setupFailed"));
    }
  }

  async function handleVerify2FASetup() {
    if (!twoFAVerifyCode.trim()) return;
    twoFactorLoading = true;
    const result = await authService.enable2FA(twoFAVerifyCode.trim());
    twoFactorLoading = false;
    if (result.success) {
      twoFactorEnabled = true;
      twoFAStep = "done";
    } else {
      toast.error(result.message || $t("settingsPage.twoFactor.codeError"));
    }
  }

  async function handleDisable2FA() {
    twoFactorLoading = true;
    const result = await authService.disable2FA();
    twoFactorLoading = false;
    if (result.success) {
      twoFactorEnabled = false;
      toast.success($t("settingsPage.twoFactor.disabled"));
    } else {
      toast.error(result.message || $t("settingsPage.twoFactor.disableFailed"));
    }
  }

  function close2FAModal() {
    show2FASetupModal = false;
    twoFASetupData = null;
    twoFAVerifyCode = "";
    twoFAStep = "qr";
  }
</script>

<div class="settings">
  <!-- 侧边栏 -->
  <nav class="sidebar">
    {#each sections as section (section.id)}
      <button
        class="nav-item"
        class:active={activeSection === section.id}
        onclick={() => (activeSection = section.id)}
      >
        <Icon icon={section.icon} width="20" />
        <span>{section.name}</span>
      </button>
    {/each}
  </nav>

  <!-- 内容区 -->
  <main class="content">
    <!-- 账户设置 -->
    {#if activeSection === "account"}
      <div class="section">
        <h2>{$t("settingsPage.account.title")}</h2>
        <div class="card">
          <div class="user-info">
            <div class="avatar" role="button" tabindex="0" title={$t("settingsPage.account.clickToChangeAvatar")}
              onclick={() => avatarInput?.click()}
              onkeydown={(e) => { if (e.key === 'Enter') avatarInput?.click(); }}>
              <img class="avatar-image" src={getAvatarUrl(accountUser)} alt="avatar" />
              <div class="avatar-overlay">
                <Icon icon="mdi:camera" width="20" />
              </div>
              <input
                bind:this={avatarInput}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                style="display:none"
                onchange={handleAvatarUpload}
              />
            </div>
            <div class="info">
              <h3>{accountUser?.username || $t("settingsPage.account.loading")}</h3>
              <p>{accountUser?.role === "admin" ? $t("settingsPage.account.admin") : $t("settingsPage.account.user")}</p>
            </div>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.account.basicInfo")}</h3>
          <div class="info-display">
            <div class="info-row">
              <span class="info-label">{$t("settingsPage.account.username")}</span>
              <span class="info-value">{accountUser?.username || ""}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("settingsPage.account.createdAt")}</span>
              <span class="info-value">{accountUser?.created_at ? new Date(accountUser.created_at).toLocaleDateString("zh-CN") : ""}</span>
            </div>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.account.security")}</h3>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.account.changePassword")}</strong>
              <p>{$t("settingsPage.account.changePasswordDesc")}</p>
            </div>
            <Button variant="secondary" onclick={() => (showPasswordModal = true)}>{$t("settingsPage.account.change")}</Button>
          </div>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.account.twoFactor")}</strong>
              <p>{twoFactorEnabled ? $t("settingsPage.account.twoFactorEnabled") : $t("settingsPage.account.twoFactorDisabled")}</p>
            </div>
            {#if twoFactorEnabled}
              <Button variant="secondary" disabled={twoFactorLoading} onclick={handleDisable2FA}>
                {twoFactorLoading ? $t("settingsPage.account.processing") : $t("settingsPage.account.disable")}
              </Button>
            {:else}
              <Button variant="primary" disabled={twoFactorLoading} onclick={handleSetup2FA}>
                {twoFactorLoading ? $t("settingsPage.account.processing") : $t("settingsPage.account.enable")}
              </Button>
            {/if}
          </div>
        </div>

        <!-- 用户管理 -->
        <div class="card" style="margin-top: 16px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
            <h3>{$t("settingsPage.users.title")}</h3>
            <Button variant="primary" onclick={() => (showCreateUserModal = true)}>
              <Icon icon="mdi:account-plus" width="18" style="margin-right: 4px;" />
              {$t("settingsPage.users.createUser")}
            </Button>
          </div>

          {#if usersLoading}
            <div style="display: flex; justify-content: center; padding: 40px;">
              <Spinner />
            </div>
          {:else}
            <div class="users-list">
              {#each managedUsers as u (u.id)}
                <div class="user-row">
                  <div class="user-row-left">
                    <div class="user-avatar-sm">
                      <img src={getAvatarUrl(u)} alt={u.nickname || u.username} />
                    </div>
                    <div class="user-info-col">
                      <div class="user-name-row">
                        <span class="user-name">{u.nickname || u.username}</span>
                        {#if u.is_online}
                          <span class="online-dot" title={$t("settingsPage.users.online")}></span>
                        {/if}
                      </div>
                      <span class="user-meta">@{u.username} · {u.id === userStore.user?.id ? $t("settingsPage.users.currentUser") : `${$t("settingsPage.users.lastLogin")}: ${formatDate(u.last_login)}`}</span>
                    </div>
                  </div>
                  <div class="user-row-actions">
                    <Button variant="ghost" size="sm" onclick={() => openResetPassword(u)}>
                      <Icon icon="mdi:key" width="16" />
                    </Button>
                    {#if u.id !== userStore.user?.id}
                      <Button variant="ghost" size="sm" onclick={() => confirmDeleteUser(u)}>
                        <Icon icon="mdi:delete" width="16" />
                      </Button>
                    {/if}
                  </div>
                </div>
              {/each}
              {#if managedUsers.length === 0}
                <div style="text-align: center; padding: 40px; color: var(--text-secondary);">
                  {$t("settingsPage.users.noUsers")}
                </div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- 语言设置 -->
        <div class="card" style="margin-top: 16px;">
          <h3><Icon icon="mdi:translate" width="20" style="margin-right: 4px; vertical-align: middle;" />{$t("settingsPage.sidebar.language")}</h3>
          <LanguageSettings showLanguage={true} showMirror={false} />
        </div>

      </div>

      <!-- 存储设置 -->
    {:else if activeSection === "storage"}
      <div class="section">
        <h2>{$t("settingsPage.storage.title")}</h2>

        {#each storageInfo as disk}
          <div class="card">
            <div class="disk-header">
              <div class="disk-info">
                <Icon icon="mdi:harddisk" width="24" />
                <div>
                  <h3>{disk.name}</h3>
                  <span class="disk-path">{disk.path} · {disk.type}</span>
                </div>
              </div>
              <span class="disk-usage">{formatSize(disk.used)} / {formatSize(disk.total)}</span>
            </div>
            <Progress
              value={disk.used}
              max={disk.total}
              variant={disk.used / disk.total > 0.9
                ? "error"
                : disk.used / disk.total > 0.7
                  ? "warning"
                  : "default"}
              showLabel
            />
          </div>
        {/each}
      </div>

      <!-- 外观设置 -->
    {:else if activeSection === "appearance"}
      <div class="section">
        <h2>{$t("settingsPage.appearance.title")}</h2>

        <div class="card">
          <h3>{$t("settingsPage.appearance.wallpaper")}</h3>

          <!-- 壁纸分类标签 -->
          <div class="wallpaper-tabs">
            <button
              class="wallpaper-tab"
              class:active={wallpaperTab === "static"}
              onclick={() => (wallpaperTab = "static")}
            >
              <Icon icon="mdi:image" width="16" />
              {$t("settingsPage.appearance.static")}
            </button>
            <button
              class="wallpaper-tab"
              class:active={wallpaperTab === "webgl"}
              onclick={() => (wallpaperTab = "webgl")}
            >
              <Icon icon="mdi:cube-outline" width="16" />
              {$t("settingsPage.appearance.webgl")}
            </button>
            <button
              class="wallpaper-tab"
              class:active={wallpaperTab === "lottie"}
              onclick={() => (wallpaperTab = "lottie")}
            >
              <Icon icon="mdi:animation-play" width="16" />
              {$t("settingsPage.appearance.animation")}
            </button>
          </div>

          <!-- 静态壁纸 -->
          {#if wallpaperTab === "static"}
            <div class="wallpaper-grid">
              {#if wallpaper.index?.static}
                {#each wallpaper.index.static as wp}
                  <button
                    class="wallpaper-option"
                    class:active={wallpaper.currentId === wp.id}
                    onclick={() => selectWallpaper(wp)}
                    title={wp.name}
                  >
                    <img
                      src="/wallpapers/{wp.thumbnail}"
                      alt={wp.name}
                      onerror={(e) =>
                        ((e.currentTarget as HTMLImageElement).src = "/icons/image.svg")}
                    />
                    <span class="wallpaper-name">{wp.name}</span>
                  </button>
                {/each}
              {:else}
                <div class="loading-placeholder">{$t("common.loading")}</div>
              {/if}
              <button class="wallpaper-option upload" onclick={openCustomWallpaperModal}>
                <Icon icon="mdi:plus" width="24" />
                <span>{$t("settingsPage.appearance.custom")}</span>
              </button>
            </div>
          {/if}

          <!-- WebGL 壁纸 -->
          {#if wallpaperTab === "webgl"}
            <div class="wallpaper-info-hint">
              <Icon icon="mdi:information-outline" width="16" />
              <span>{$t("settingsPage.appearance.webglHint")}</span>
            </div>
            <div class="effect-list">
              {#if wallpaper.index?.webgl}
                {#each wallpaper.index.webgl as wp}
                  <button
                    class="effect-item"
                    class:active={wallpaper.currentId === wp.id}
                    onclick={() => selectWallpaper(wp)}
                  >
                    <Icon icon="mdi:cube-outline" width="20" />
                    <span>{wp.name}</span>
                  </button>
                {/each}
              {:else}
                <div class="loading-placeholder">{$t("common.loading")}</div>
              {/if}
            </div>
          {/if}

          <!-- Lottie 动画壁纸 -->
          {#if wallpaperTab === "lottie"}
            <div class="wallpaper-info-hint">
              <Icon icon="mdi:information-outline" width="16" />
              <span>{$t("settingsPage.appearance.lottieHint")}</span>
            </div>
            <div class="effect-list">
              {#if wallpaper.index?.lottie}
                {#each wallpaper.index.lottie as wp}
                  <button
                    class="effect-item"
                    class:active={wallpaper.currentId === wp.id}
                    onclick={() => selectWallpaper(wp)}
                  >
                    <Icon icon="mdi:animation-play" width="20" />
                    <span>{wp.name}</span>
                  </button>
                {/each}
              {:else}
                <div class="loading-placeholder">{$t("common.loading")}</div>
              {/if}
            </div>
          {/if}
        </div>

        <div class="card">
          <h3>{$t("settingsPage.appearance.theme")}</h3>
          <div class="theme-options">
            <button
              class="theme-option"
              class:active={currentThemeMode === "light"}
              onclick={() => setTheme("light")}
            >
              <div class="preview light"></div>
              <span>{$t("settingsPage.appearance.light")}</span>
            </button>
            <button
              class="theme-option"
              class:active={currentThemeMode === "dark"}
              onclick={() => setTheme("dark")}
            >
              <div class="preview dark"></div>
              <span>{$t("settingsPage.appearance.dark")}</span>
            </button>
            <button
              class="theme-option"
              class:active={currentThemeMode === "auto"}
              onclick={() => setTheme("auto")}
            >
              <div class="preview auto"></div>
              <span>{$t("settingsPage.appearance.auto")}</span>
            </button>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.appearance.accentColor")}</h3>
          <div class="color-options">
            {#each ["#4a90d9", "#52c41a", "#faad14", "#ff4d4f", "#722ed1", "#eb2f96"] as color}
              <button
                class="color-option"
                class:active={accentColor === color}
                style="background: {color}"
                onclick={() => theme.setAccent(color)}
              ></button>
            {/each}
          </div>
        </div>
      </div>

      <!-- 自定义壁纸弹窗 -->
      <Modal
        open={showCustomWallpaperModal}
        title={$t("settingsPage.appearance.customWallpaper")}
        onclose={() => (showCustomWallpaperModal = false)}
      >
        <div class="custom-wallpaper-form">
          <div class="form-group">
            <label>{$t("settingsPage.appearance.loadFromNetwork")}</label>
            <Input type="url" placeholder={$t("settingsPage.appearance.inputUrlPlaceholder")} bind:value={customWallpaperUrl} />
          </div>

          <div class="form-divider">{$t("settingsPage.appearance.or")}</div>

          <div class="form-group">
            <label>{$t("settingsPage.appearance.uploadFromLocal")}</label>
            <input
              type="file"
              accept="image/*,video/*"
              onchange={handleCustomWallpaperFile}
              class="file-input"
            />
          </div>

          {#if customWallpaperFile}
            <div class="file-preview">
              {$t("settingsPage.appearance.selectedFile")}: {customWallpaperFile.name}
            </div>
          {/if}
        </div>

        {#snippet footer()}
          <Button variant="ghost" onclick={() => (showCustomWallpaperModal = false)}>{$t("common.cancel")}</Button>
          <Button variant="primary" onclick={applyCustomWallpaper}>{$t("settingsPage.appearance.apply")}</Button>
        {/snippet}
      </Modal>

      <!-- 通知设置 -->
    {:else if activeSection === "notification"}
      <div class="section">
        <h2>{$t("settingsPage.notification.title")}</h2>
        <div class="card">
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.notification.enableNotifications")}</strong>
              <p>{$t("settingsPage.notification.enableNotificationsDesc")}</p>
            </div>
            <Switch
              checked={settings.notifications.enabled}
              onchange={(val) => settings.updateNotifications({ enabled: val })}
            />
          </div>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.notification.desktopNotification")}</strong>
              <p>{$t("settingsPage.notification.desktopNotificationDesc")}</p>
            </div>
            <Switch
              checked={settings.notifications.desktop}
              onchange={(val) => settings.updateNotifications({ desktop: val })}
            />
          </div>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.notification.sound")}</strong>
              <p>{$t("settingsPage.notification.soundDesc")}</p>
            </div>
            <Switch
              checked={settings.notifications.sound}
              onchange={(val) => settings.updateNotifications({ sound: val })}
            />
          </div>
        </div>

        <!-- 泡泡通知设置 -->
        <div class="card">
          <div class="card-header">
            <h3>🫧 {$t("settingsPage.notification.bubbleNotification")}</h3>
          </div>
          <p class="card-description">{$t("settingsPage.notification.bubbleDescription")}</p>

          <div class="form-group" style="margin-top: 16px;">
            <label class="form-label">{$t("settingsPage.notification.displayMode")}</label>
            <div class="radio-group vertical">
              <label class="radio-option">
                <input
                  type="radio"
                  name="bubble-mode"
                  value="auto"
                  checked={notificationBubbleStore.mode === "auto"}
                  onchange={() => notificationBubbleStore.setMode("auto")}
                />
                <div class="radio-content">
                  <span>{$t("settingsPage.notification.followWallpaper")}</span>
                  <p class="radio-hint">{$t("settingsPage.notification.followWallpaperHint")}</p>
                </div>
              </label>
              <label class="radio-option">
                <input
                  type="radio"
                  name="bubble-mode"
                  value="always"
                  checked={notificationBubbleStore.mode === "always"}
                  onchange={() => notificationBubbleStore.setMode("always")}
                />
                <div class="radio-content">
                  <span>{$t("settingsPage.notification.alwaysOn")}</span>
                  <p class="radio-hint">{$t("settingsPage.notification.alwaysOnHint")}</p>
                </div>
              </label>
              <label class="radio-option">
                <input
                  type="radio"
                  name="bubble-mode"
                  value="never"
                  checked={notificationBubbleStore.mode === "never"}
                  onchange={() => notificationBubbleStore.setMode("never")}
                />
                <div class="radio-content">
                  <span>{$t("settingsPage.notification.alwaysOff")}</span>
                  <p class="radio-hint">{$t("settingsPage.notification.alwaysOffHint")}</p>
                </div>
              </label>
            </div>
          </div>

          <div class="form-row" style="margin-top: 16px;">
            <div class="form-group">
              <label class="form-label">{$t("settingsPage.notification.maxBubbles")}</label>
              <Select
                value={String(notificationBubbleStore.maxBubbles)}
                options={[
                  { value: "3", label: $t("settingsPage.notification.bubbleCount", { values: { n: 3 } }) },
                  { value: "5", label: $t("settingsPage.notification.bubbleCount", { values: { n: 5 } }) },
                  { value: "7", label: $t("settingsPage.notification.bubbleCount", { values: { n: 7 } }) },
                  { value: "10", label: $t("settingsPage.notification.bubbleCount", { values: { n: 10 } }) },
                ]}
                onchange={(val) => notificationBubbleStore.setMaxBubbles(parseInt(val))}
              />
            </div>
            <div class="form-group">
              <label class="form-label">{$t("settingsPage.notification.autoHideTime")}</label>
              <Select
                value={String(notificationBubbleStore.autoHideSeconds)}
                options={[
                  { value: "15", label: $t("settingsPage.notification.seconds", { values: { n: 15 } }) },
                  { value: "30", label: $t("settingsPage.notification.seconds", { values: { n: 30 } }) },
                  { value: "60", label: $t("settingsPage.notification.minute") },
                  { value: "120", label: $t("settingsPage.notification.minutes", { values: { n: 2 } }) },
                ]}
                onchange={(val) => notificationBubbleStore.setAutoHideSeconds(parseInt(val))}
              />
            </div>
          </div>

          <div class="bubble-status" style="margin-top: 12px;">
            <Icon
              icon={notificationBubbleStore.isEnabled ? "mdi:check-circle" : "mdi:close-circle"}
              width="16"
            />
            <span>{$t("settingsPage.notification.currentStatus")}：{notificationBubbleStore.isEnabled ? $t("settingsPage.notification.enabled") : $t("settingsPage.notification.disabled")}</span>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.notification.position")}</h3>
          <div class="radio-group">
            {#each [{ value: "top-right", label: $t("settingsPage.notification.topRight") }, { value: "top-left", label: $t("settingsPage.notification.topLeft") }, { value: "bottom-right", label: $t("settingsPage.notification.bottomRight") }, { value: "bottom-left", label: $t("settingsPage.notification.bottomLeft") }] as option}
              <label class="radio-option">
                <input
                  type="radio"
                  name="notification-position"
                  value={option.value}
                  checked={settings.notifications.position === option.value}
                  onchange={() => settings.updateNotifications({ position: option.value as any })}
                />
                <span>{option.label}</span>
              </label>
            {/each}
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.notification.duration")}</h3>
          <div class="form-group">
            <Select
              value={String(settings.notifications.duration)}
              options={[
                { value: "3", label: $t("settingsPage.notification.seconds", { values: { n: 3 } }) },
                { value: "5", label: $t("settingsPage.notification.seconds", { values: { n: 5 } }) },
                { value: "10", label: $t("settingsPage.notification.seconds", { values: { n: 10 } }) },
                { value: "0", label: $t("settingsPage.notification.noAutoClose") },
              ]}
              onchange={(val) => settings.updateNotifications({ duration: parseInt(val) })}
            />
          </div>
        </div>

        <!-- 推送渠道 -->
        <div class="card">
          <div class="card-header">
            <h3>{$t("settingsPage.notification.pushChannels")}</h3>
            <Button variant="ghost" size="sm" onclick={openAddChannelModal}>
              <Icon icon="mdi:plus" />
              {$t("settingsPage.notification.addChannel")}
            </Button>
          </div>
          <p class="card-description">
            {$t("settingsPage.notification.pushChannelsDesc")}
          </p>

          {#if channelsLoading}
            <div class="loading-state" style="padding: 20px;">
              <Spinner size="md" />
              <p>{$t("common.loading")}</p>
            </div>
          {:else if notificationChannels.length === 0}
            <div
              class="empty-state"
              style="padding: 20px; text-align: center; color: var(--text-secondary);"
            >
              <Icon icon="mdi:bell-off-outline" width="48" style="opacity: 0.5" />
              <p>{$t("settingsPage.notification.noPushChannels")}</p>
              <p style="font-size: 12px; margin-top: 4px;">{$t("settingsPage.notification.addChannelHint")}</p>
            </div>
          {:else}
            <div class="channel-list">
              {#each notificationChannels as channel}
                <div class="channel-item">
                  <div class="channel-info">
                    <div
                      class="channel-icon"
                      style="background: {channelTypeInfo[channel.type]?.color ||
                        '#666'}20; color: {channelTypeInfo[channel.type]?.color || '#666'}"
                    >
                      <Icon icon={channelTypeInfo[channel.type]?.icon || "mdi:bell"} width="20" />
                    </div>
                    <div class="channel-details">
                      <strong>{channel.name}</strong>
                      <span class="channel-type"
                        >{channelTypeInfo[channel.type]?.name || channel.type}</span
                      >
                    </div>
                  </div>
                  <div class="channel-actions">
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => testChannel(channel)}
                      title={$t("settingsPage.notification.sendTestNotification")}
                    >
                      <Icon icon="mdi:send" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => openEditChannelModal(channel)}
                      title={$t("settingsPage.notification.edit")}
                    >
                      <Icon icon="mdi:pencil" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => deleteChannel(channel)}
                      title={$t("settingsPage.notification.delete")}
                    >
                      <Icon icon="mdi:delete" />
                    </Button>
                    <Switch
                      checked={channel.enabled}
                      onchange={() => toggleChannelEnabled(channel)}
                    />
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- 安全设置 -->
    {:else if activeSection === "security"}
      <div class="section">
        <h2>{$t("settingsPage.security.title")}</h2>

        <!-- 远程访问 -->
        <div class="card">
          <h3>{$t("settingsPage.security.remoteAccess")}</h3>
          {#if remoteAccessLoading}
            <div class="loading-state" style="padding: 20px;">
              <Spinner size="md" />
              <p>{$t("common.loading")}</p>
            </div>
          {:else if remoteAccessError && !remoteAccessSettings}
            <div class="error-state" style="padding: 20px;">
              <Icon icon="mdi:alert-circle" width="32" />
              <p>{remoteAccessError}</p>
              <Button onclick={loadRemoteAccessSettings}>{$t("settingsPage.security.retry")}</Button>
            </div>
          {:else if remoteAccessSettings}
            <div class="setting-row">
              <div>
                <strong>{$t("settingsPage.security.sshService")}</strong>
                <p>{$t("settingsPage.security.sshServiceDesc")}</p>
              </div>
              <Switch
                checked={remoteAccessSettings.ssh_enabled}
                onclick={(e) => {
                  e.preventDefault();
                  openSecurityConfirm("ssh", !remoteAccessSettings!.ssh_enabled);
                }}
              />
            </div>
            {#if remoteAccessSettings.ssh_enabled && remoteAccessSettings.ssh_running}
              <div class="status-info" style="margin-top: 8px; padding-left: 16px;">
                <div class="info-row">
                  <span>{$t("settingsPage.network.connectionStatus")}</span>
                  <span class="status-badge success">{$t("settingsPage.security.running")}</span>
                </div>
                <div class="info-row">
                  <span>{$t("settingsPage.security.port")}</span>
                  <span>{remoteAccessSettings.ssh_port}</span>
                </div>
              </div>
            {/if}

            <div class="setting-row" style="margin-top: 16px;">
              <div>
                <strong>{$t("settingsPage.security.webTerminal")}</strong>
                <p>{$t("settingsPage.security.webTerminalDesc")}</p>
              </div>
              <Switch
                checked={remoteAccessSettings.terminal_enabled}
                onclick={(e) => {
                  e.preventDefault();
                  openSecurityConfirm("terminal", !remoteAccessSettings!.terminal_enabled);
                }}
              />
            </div>
          {/if}
        </div>

        <div class="card warning-card">
          <div class="warning-header">
            <Icon icon="mdi:alert" width="24" />
            <strong>{$t("settingsPage.security.securityWarning")}</strong>
          </div>
          <div class="warning-content">
            <p>{$t("settingsPage.security.securityWarningText1")}</p>
            <p>{$t("settingsPage.security.securityWarningText2")}</p>
            <p>{$t("settingsPage.security.securityWarningText3")}</p>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.security.power")}</h3>
          <div class="power-actions" style="margin-top: 16px;">
            <Button variant="secondary" onclick={() => confirmPower("restart")}>
              <Icon icon="mdi:restart" width="18" />
              {$t("settingsPage.security.restart")}
            </Button>
            <Button variant="danger" onclick={() => confirmPower("shutdown")}>
              <Icon icon="mdi:power" width="18" />
              {$t("settingsPage.security.shutdown")}
            </Button>
          </div>
        </div>

        {#if showPowerConfirmModal}
          <Modal title={powerAction === "restart" ? $t("settingsPage.security.restart") : $t("settingsPage.security.shutdown")} onclose={() => showPowerConfirmModal = false}>
            <p>{powerAction === "restart" ? $t("settingsPage.security.restartConfirm") : $t("settingsPage.security.shutdownConfirm")}</p>
            <div style="display: flex; gap: 12px; justify-content: flex-end; margin-top: 16px;">
              <Button variant="secondary" onclick={() => showPowerConfirmModal = false}>{$t("common.cancel")}</Button>
              <Button variant={powerAction === "restart" ? "primary" : "danger"} onclick={executePowerAction} disabled={powerLoading}>
                {#if powerLoading}
                  <Spinner size="sm" />
                {:else}
                  {powerAction === "restart" ? $t("settingsPage.security.restart") : $t("settingsPage.security.shutdown")}
                {/if}
              </Button>
            </div>
          </Modal>
        {/if}

        <div class="card danger-card">
          <h3>{$t("settingsPage.security.dangerZone")}</h3>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.security.factoryReset")}</strong>
              <p>{$t("settingsPage.security.factoryResetDesc")}</p>
            </div>
            <Button variant="danger" onclick={openFactoryResetModal}>{$t("settingsPage.security.factoryResetBtn")}</Button>
          </div>
        </div>
      </div>

      <!-- 网络 -->
    {:else if activeSection === "network"}
      <div class="section">
        <h2>{$t("settingsPage.network.title")}</h2>
        
        <!-- 网络连接 -->
        <div class="card">
          <h3>{$t("settingsPage.network.connection")}</h3>
          <div class="form-group">
            <label>{$t("settingsPage.network.hostname")}</label>
            <Input bind:value={hostname} placeholder={$t("settingsPage.network.hostname")} />
          </div>
          <div class="setting-row">
            <div>
              <strong>{$t("settingsPage.network.dhcp")}</strong>
              <p>{$t("settingsPage.network.dhcpDesc")}</p>
            </div>
            <Switch bind:checked={dhcpEnabled} />
          </div>
          {#if !dhcpEnabled}
            <div class="form-group">
              <label>{$t("settingsPage.network.ipAddress")}</label>
              <Input bind:value={ipAddress} placeholder="192.168.1.100" />
            </div>
            <div class="form-group">
              <label>{$t("settingsPage.network.gateway")}</label>
              <Input bind:value={gateway} placeholder="192.168.1.1" />
            </div>
            <div class="form-group">
              <label>{$t("settingsPage.network.dns")}</label>
              <Input bind:value={dns} placeholder="8.8.8.8" />
            </div>
          {/if}
          <div class="info-row" style="margin-top: 12px;">
            <span>{$t("settingsPage.network.connectionStatus")}</span>
            <span class="status-badge success">{$t("settingsPage.network.connected")}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.network.ipAddress")}</span>
            <span>192.168.1.100</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.network.macAddress")}</span>
            <span>00:1A:2B:3C:4D:5E</span>
          </div>
        </div>

        <!-- 代理设置 -->
        <div class="card">
          <h3>{$t("settingsPage.network.proxy")}</h3>
          <div class="form-group">
            <label>{$t("settingsPage.network.proxyMode")}</label>
            <Select
              bind:value={proxyMode}
              options={[
                { value: "off", label: $t("settingsPage.network.proxyOff") },
                { value: "manual", label: $t("settingsPage.network.proxyManual") },
                { value: "pac", label: $t("settingsPage.network.proxyPac") },
              ]}
            />
          </div>

          {#if proxyMode === "manual"}
            <div class="form-group">
              <label>{$t("settingsPage.network.httpProxy")}</label>
              <Input bind:value={httpProxy} placeholder="http://host:port" />
            </div>
            <div class="form-group">
              <label>{$t("settingsPage.network.httpsProxy")}</label>
              <Input bind:value={httpsProxy} placeholder="http://host:port" />
            </div>
            <div class="form-group">
              <label>{$t("settingsPage.network.socks5Proxy")}</label>
              <Input bind:value={socks5Proxy} placeholder="socks5://host:port" />
            </div>
            <div class="form-group">
              <label>{$t("settingsPage.network.noProxyAddresses")}</label>
              <Input bind:value={noProxy} placeholder="localhost,127.0.0.1,*.local" />
              <p class="form-hint">{$t("settingsPage.network.noProxyHint")}</p>
            </div>

            <div class="setting-row">
              <div>
                <strong>{$t("settingsPage.network.authentication")}</strong>
                <p>{$t("settingsPage.network.authenticationDesc")}</p>
              </div>
              <Switch bind:checked={proxyAuthEnabled} />
            </div>
            {#if proxyAuthEnabled}
              <div class="form-group">
                <label>{$t("settingsPage.network.username")}</label>
                <Input bind:value={proxyUsername} placeholder={$t("settingsPage.network.username")} />
              </div>
              <div class="form-group">
                <label>{$t("settingsPage.network.password")}</label>
                <Input bind:value={proxyPassword} type="password" placeholder={$t("settingsPage.network.password")} />
              </div>
            {/if}

            <!-- Docker 加速设置 -->
            <div style="margin-top: 16px; padding-top: 16px; border-top: 1px solid var(--border-color, #e5e5e5);">
              <div class="form-group">
                <label>{$t("settingsPage.network.dockerMirror")}</label>
                <Input bind:value={dockerMirror} placeholder={$t("settingsPage.network.dockerMirrorPlaceholder")} />
                <p class="form-hint">{$t("settingsPage.network.dockerMirrorHint")}</p>
              </div>
              <div class="setting-row">
                <div>
                  <strong>{$t("settingsPage.network.dockerProxy")}</strong>
                  <p>{$t("settingsPage.network.dockerProxyDesc")}</p>
                </div>
                <Switch bind:checked={dockerProxyEnabled} />
              </div>
            </div>
          {:else if proxyMode === "pac"}
            <div class="form-group">
              <label>{$t("settingsPage.network.pacAddress")}</label>
              <Input bind:value={pacUrl} placeholder="http://example.com/proxy.pac" />
            </div>
          {/if}

          {#if proxyMode !== "off"}
            <div class="form-actions" style="margin-top: 16px; gap: 8px;">
              <Button variant="primary" onclick={saveProxySettings}>{$t("settingsPage.network.saveSettings")}</Button>
              <Button variant="secondary" onclick={testProxyConnection} disabled={proxyTesting}>
                {#if proxyTesting}
                  <Spinner size="sm" />
                  {$t("settingsPage.network.testing")}
                {:else}
                  {$t("settingsPage.network.testConnection")}
                {/if}
              </Button>
            </div>
            {#if proxyTestResult}
              <div class="proxy-test-result" class:success={proxyTestResult.success} class:error={!proxyTestResult.success}>
                <Icon icon={proxyTestResult.success ? "mdi:check-circle" : "mdi:alert-circle"} width="16" />
                {proxyTestResult.message}
              </div>
            {/if}
          {/if}
        </div>
      </div>

      <!-- 关于 -->
    {:else if activeSection === "about"}
      <div class="section">
        <h2>{$t("settingsPage.about.title")}</h2>
        <div class="card">
          <div class="about-info">
            <div class="logo-wrapper">
              <Icon icon="mdi:server" width="64" />
            </div>
            <h3>RDE</h3>
            <p class="version">{$t("settingsPage.about.version")} {currentVersion}</p>
            <p class="desc">{$t("settingsPage.about.description")}</p>
          </div>
        </div>

        <div class="card">
          <h3>{$t("settingsPage.about.systemInfo")}</h3>
          <div class="info-row">
            <span>{$t("settingsPage.about.deviceName")}</span>
            <span>{systemInfo.deviceName}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.about.uptime")}</span>
            <span>{systemInfo.uptime}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.about.processor")}</span>
            <span>{systemInfo.cpu}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.about.memory")}</span>
            <span>{systemInfo.memory}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.about.kernelVersion")}</span>
            <span>{systemInfo.kernel}</span>
          </div>
          <div class="info-row">
            <span>{$t("settingsPage.about.architecture")}</span>
            <span>{systemInfo.arch}</span>
          </div>
        </div>
      </div>
    {/if}
  </main>
</div>

<!-- 修改密码弹窗 -->
<Modal bind:open={showPasswordModal} title={$t("settingsPage.modals.changePassword")} size="sm" showFooter>
  <div class="form-group">
    <label>{$t("settingsPage.modals.currentPassword")}</label>
    <Input type="password" bind:value={currentPassword} placeholder={$t("settingsPage.modals.inputCurrentPassword")} />
  </div>
  <div class="form-group">
    <label>{$t("settingsPage.modals.newPassword")}</label>
    <Input type="password" bind:value={newPassword} placeholder={$t("settingsPage.modals.inputNewPassword")} />
  </div>
  <div class="form-group">
    <label>{$t("settingsPage.modals.confirmPassword")}</label>
    <Input type="password" bind:value={confirmPassword} placeholder={$t("settingsPage.modals.inputNewPasswordAgain")} />
  </div>
  {#snippet footer()}
    <Button variant="secondary" onclick={() => (showPasswordModal = false)}>{$t("settingsPage.modals.cancel")}</Button>
    <Button variant="primary" onclick={handlePasswordChange}>{$t("settingsPage.modals.confirmChange")}</Button>
  {/snippet}
</Modal>

<!-- 两步验证设置弹窗 -->
<Modal bind:open={show2FASetupModal} title={$t("settingsPage.modals.setup2FA")} size="md" showFooter>
  {#if twoFAStep === "qr"}
    <div class="twofa-setup">
      <p class="twofa-desc">
        {$t("settingsPage.modals.scan2FADesc")}
      </p>

      {#if twoFASetupData?.qr_code_url}
        <div class="twofa-qr">
          <img
            src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data={encodeURIComponent(twoFASetupData.qr_code_url)}"
            alt="2FA QR Code"
            width="200"
            height="200"
          />
        </div>
      {/if}

      {#if twoFASetupData?.secret}
        <div class="twofa-secret">
          <label>{$t("settingsPage.modals.manualInputKey")}</label>
          <code>{twoFASetupData.secret}</code>
        </div>
      {/if}

      <div class="twofa-verify">
        <label>{$t("settingsPage.twoFactor.enterCodeToConfirm")}</label>
        <div class="twofa-code-input">
          <Input
            bind:value={twoFAVerifyCode}
            placeholder="000000"
            maxlength={6}
          />
          <Button variant="primary" disabled={twoFactorLoading || twoFAVerifyCode.length < 6} onclick={handleVerify2FASetup}>
            {twoFactorLoading ? $t("settingsPage.twoFactor.verifying") : $t("settingsPage.twoFactor.verifyAndEnable")}
          </Button>
        </div>
      </div>
    </div>
  {:else if twoFAStep === "done"}
    <div class="twofa-setup">
      <div class="twofa-success">
        <Icon icon="mdi:check-circle" width="48" />
        <h3>{$t("settingsPage.twoFactor.enabled")}</h3>
        <p>{$t("settingsPage.twoFactor.nextLoginNeedCode")}</p>
      </div>

      {#if twoFASetupData?.backup_codes?.length}
        <div class="twofa-backup">
          <div class="twofa-backup-header">
            <h4>{$t("settingsPage.twoFactor.backupCodes")}</h4>
            <button class="twofa-copy-btn" onclick={() => {
              const text = twoFASetupData?.backup_codes?.join('\n') || '';
              navigator.clipboard.writeText(text);
              backupCodesCopied = true;
              setTimeout(() => backupCodesCopied = false, 2000);
            }}>
              <Icon icon={backupCodesCopied ? 'mdi:check' : 'mdi:content-copy'} width="16" />
              {backupCodesCopied ? $t("settingsPage.twoFactor.copied") : $t("settingsPage.twoFactor.copyAll")}
            </button>
          </div>
          <p class="twofa-backup-warn">{$t("settingsPage.twoFactor.backupCodesWarn")}</p>
          <pre class="twofa-backup-codes-block">{twoFASetupData.backup_codes.join('\n')}</pre>
        </div>
      {/if}
    </div>
  {/if}
  {#snippet footer()}
    {#if twoFAStep === "qr"}
      <Button variant="secondary" onclick={close2FAModal}>{$t("settingsPage.modals.cancel")}</Button>
    {:else}
      <Button variant="primary" onclick={close2FAModal}>{$t("settingsPage.modals.done")}</Button>
    {/if}
  {/snippet}
</Modal>

<!-- 安全设置确认弹窗 -->
<Modal bind:open={showSecurityConfirmModal} title={$t("settingsPage.modals.securityConfirm")} size="sm" showFooter={true}>
  <div class="security-confirm-content">
    {#if pendingSecurityAction}
      <div class="warning-icon" style="font-size: 48px; text-align: center; margin-bottom: 16px;">
        {#if pendingSecurityAction.enabled}
          ⚠️
        {:else}
          🔒
        {/if}
      </div>
      <p class="warning-text" style="text-align: center; margin-bottom: 16px;">
        {#if pendingSecurityAction.enabled}
          {#if pendingSecurityAction.type === "ssh"}
            {$t("settingsPage.modals.enableSshWarn")}
          {:else}
            {$t("settingsPage.modals.enableWebTerminalWarn")}
          {/if}
          <br />{$t("settingsPage.modals.ensureUnderstandRisk")}
        {:else if pendingSecurityAction.type === "ssh"}
          {$t("settingsPage.modals.confirmDisableSsh")}
        {:else}
          {$t("settingsPage.modals.confirmDisableWebTerminal")}
        {/if}
      </p>
    {/if}

    <div class="form-group">
      <label>{$t("settingsPage.modals.enterPasswordToConfirm")}</label>
      <Input type="password" bind:value={securityConfirmPassword} placeholder={$t("settingsPage.modals.enterYourPassword")} />
    </div>

    {#if securityConfirmError}
      <Alert type="error">{securityConfirmError}</Alert>
    {/if}
  </div>

  {#snippet footer()}
    <Button
      variant="secondary"
      onclick={() => (showSecurityConfirmModal = false)}
      disabled={securityConfirmLoading}
    >
      {$t("settingsPage.modals.cancel")}
    </Button>
    <Button
      variant={pendingSecurityAction?.enabled ? "warning" : "primary"}
      onclick={executeSecurityUpdate}
      disabled={securityConfirmLoading}
    >
      {#if securityConfirmLoading}
        <Spinner size="sm" />
      {:else}
        {pendingSecurityAction?.enabled ? $t("settingsPage.modals.confirmEnable") : $t("settingsPage.modals.confirmDisable")}
      {/if}
    </Button>
  {/snippet}
</Modal>

<!-- 恢复出厂设置弹窗 -->
<Modal
  bind:open={showFactoryResetModal}
  title={$t("settingsPage.modals.factoryReset")}
  size="md"
  showFooter={factoryResetStep !== "processing"}
>
  {#if factoryResetStep === "warning"}
    <div class="factory-reset-content">
      <div class="warning-icon">⚠️</div>
      <p class="warning-text">
        {$t("settingsPage.modals.factoryResetWarn")}
      </p>

      <div class="warning-list">
        <h4>{$t("settingsPage.modals.dataWillBeDeleted")}</h4>
        <ul>
          <li>{$t("settingsPage.modals.deleteItem1")}</li>
          <li>{$t("settingsPage.modals.deleteItem2")}</li>
          <li>{$t("settingsPage.modals.deleteItem3")}</li>
          <li>{$t("settingsPage.modals.deleteItem4")}</li>
        </ul>
      </div>

      <div class="reset-options">
        <h4>{$t("settingsPage.modals.optionalKeep")}</h4>
        <label class="option-item">
          <input type="checkbox" bind:checked={factoryResetKeepDocker} />
          <span>{$t("settingsPage.modals.keepDocker")}</span>
        </label>
        <label class="option-item">
          <input type="checkbox" bind:checked={factoryResetKeepFiles} />
          <span>{$t("settingsPage.modals.keepFiles")}</span>
        </label>
      </div>
    </div>
  {:else if factoryResetStep === "confirm"}
    <div class="factory-reset-content">
      <div class="warning-icon">🔐</div>
      <p class="warning-text">{$t("settingsPage.modals.enterPasswordForSecurity")}</p>

      <div class="form-group">
        <label>{$t("settingsPage.modals.currentPassword")}</label>
        <Input type="password" bind:value={factoryResetPassword} placeholder={$t("settingsPage.modals.pleaseEnterPassword")} />
      </div>

      <div class="form-group">
        <label>{$t("settingsPage.modals.enterResetToConfirm")}</label>
        <Input bind:value={factoryResetConfirmText} placeholder="RESET" />
      </div>

      {#if factoryResetError}
        <Alert type="error">{factoryResetError}</Alert>
      {/if}
    </div>
  {:else if factoryResetStep === "processing"}
    <div class="factory-reset-content processing">
      <div class="spinner"></div>
      <p class="processing-text">{$t("settingsPage.modals.resettingDoNotClose")}</p>
      <div class="progress-log">
        {#each factoryResetProgress as msg}
          <p>• {msg}</p>
        {/each}
      </div>
    </div>
  {/if}

  {#snippet footer()}
    {#if factoryResetStep === "warning"}
      <Button variant="secondary" onclick={closeFactoryResetModal}>{$t("settingsPage.modals.cancel")}</Button>
      <Button variant="danger" onclick={() => (factoryResetStep = "confirm")}
        >{$t("settingsPage.modals.understandRiskContinue")}</Button
      >
    {:else if factoryResetStep === "confirm"}
      <Button variant="secondary" onclick={() => (factoryResetStep = "warning")}>{$t("settingsPage.modals.back")}</Button>
      <Button
        variant="danger"
        onclick={executeFactoryReset}
        disabled={!factoryResetPassword || factoryResetConfirmText !== "RESET"}
      >
        {$t("settingsPage.modals.confirmFactoryReset")}
      </Button>
    {/if}
  {/snippet}
</Modal>

<!-- 推送渠道编辑弹窗 -->
<Modal
  bind:open={showChannelModal}
  title={editingChannel ? $t("settingsPage.modals.editPushChannel") : $t("settingsPage.modals.addPushChannel")}
  size="md"
>
  <div class="channel-form">
    <div class="form-group">
      <label>{$t("settingsPage.modals.channelName")}</label>
      <Input bind:value={channelForm.name} placeholder={$t("settingsPage.modals.channelNamePlaceholder")} />
    </div>

    {#if !editingChannel}
      <div class="form-group">
        <label>{$t("settingsPage.modals.channelType")}</label>
        <div class="channel-type-grid">
          {#each allChannelTypes as type}
            <button
              type="button"
              class="channel-type-option"
              class:selected={channelForm.type === type}
              onclick={() => {
                channelForm.type = type;
                channelForm.config = {};
              }}
            >
              <div
                class="type-icon"
                style="background: {channelTypeInfo[type]?.color}20; color: {channelTypeInfo[type]
                  ?.color}"
              >
                <Icon icon={channelTypeInfo[type]?.icon || "mdi:bell"} width="24" />
              </div>
              <span>{channelTypeInfo[type]?.name}</span>
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <div class="form-group">
      <label>{$t("settingsPage.modals.channelConfig")}</label>
      <div class="config-fields">
        {#if channelForm.type === "email"}
          <div class="field">
            <label>{$t("settingsPage.modals.recipientEmail")}</label>
            <Input bind:value={channelForm.config.email} placeholder="your@email.com" />
          </div>
        {:else if channelForm.type === "telegram"}
          <div class="field">
            <label>Bot Token</label>
            <Input bind:value={channelForm.config.bot_token} placeholder={$t("settingsPage.modals.fromBotFather")} />
          </div>
          <div class="field">
            <label>Chat ID</label>
            <Input bind:value={channelForm.config.chat_id} placeholder={$t("settingsPage.modals.yourTelegramChatId")} />
          </div>
        {:else if channelForm.type === "webhook"}
          <div class="field">
            <label>Webhook URL</label>
            <Input bind:value={channelForm.config.url} placeholder="https://..." />
          </div>
          <div class="field">
            <label>{$t("settingsPage.modals.requestMethod")}</label>
            <Select
              value={channelForm.config.method || "POST"}
              options={[
                { value: "POST", label: "POST" },
                { value: "GET", label: "GET" },
              ]}
              onchange={(val) => (channelForm.config.method = val)}
            />
          </div>
        {:else if channelForm.type === "wechat"}
          <div class="field">
            <label>Webhook URL</label>
            <Input
              bind:value={channelForm.config.webhook_url}
              placeholder={$t("settingsPage.modals.wechatWebhook")}
            />
          </div>
        {:else if channelForm.type === "dingtalk"}
          <div class="field">
            <label>Webhook URL</label>
            <Input
              bind:value={channelForm.config.webhook_url}
              placeholder={$t("settingsPage.modals.dingtalkWebhook")}
            />
          </div>
          <div class="field">
            <label>{$t("settingsPage.modals.signSecret")}</label>
            <Input bind:value={channelForm.config.secret} placeholder={$t("settingsPage.modals.signSecretPlaceholder")} />
          </div>
        {:else if channelForm.type === "bark"}
          <div class="field">
            <label>{$t("settingsPage.modals.deviceKey")}</label>
            <Input bind:value={channelForm.config.device_key} placeholder={$t("settingsPage.modals.yourBarkDeviceKey")} />
          </div>
          <div class="field">
            <label>{$t("settingsPage.modals.serverAddress")}</label>
            <Input bind:value={channelForm.config.server_url} placeholder={$t("settingsPage.modals.defaultOfficialServer")} />
          </div>
        {/if}
      </div>
    </div>

    <div class="form-group">
      <label>{$t("settingsPage.modals.remark")}</label>
      <Input bind:value={channelForm.description} placeholder={$t("settingsPage.modals.addRemark")} />
    </div>
  </div>

  {#snippet footer()}
    <Button variant="ghost" onclick={() => (showChannelModal = false)}>{$t("settingsPage.modals.cancel")}</Button>
    <Button variant="primary" onclick={saveChannel} disabled={!channelForm.name}>
      {editingChannel ? $t("settingsPage.modals.save") : $t("settingsPage.modals.add")}
    </Button>
  {/snippet}
</Modal>

<!-- 创建用户弹窗 -->
<Modal bind:open={showCreateUserModal} title={$t("settingsPage.modals.createUser")} size="sm" showFooter>
  <div class="form-content">
    <div class="form-group">
      <label>{$t("settingsPage.modals.username")} <span style="color: var(--color-danger);">*</span></label>
      <Input bind:value={newUserUsername} placeholder={$t("settingsPage.modals.usernameHint")} />
    </div>
    <div class="form-group">
      <label>{$t("settingsPage.modals.password")} <span style="color: var(--color-danger);">*</span></label>
      <Input bind:value={newUserPassword} type="password" placeholder={$t("settingsPage.modals.passwordHint")} />
    </div>
    <div class="form-group">
      <label>{$t("settingsPage.modals.nickname")}</label>
      <Input bind:value={newUserNickname} placeholder={$t("settingsPage.modals.nicknameHint")} />
    </div>
    <Alert variant="info">
      {$t("settingsPage.modals.createUserNote")}
    </Alert>
  </div>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (showCreateUserModal = false)}>{$t("settingsPage.modals.cancel")}</Button>
    <Button variant="primary" onclick={handleCreateUser} disabled={creatingUser || !newUserUsername || !newUserPassword}>
      {#if creatingUser}
        <Spinner size="sm" />
      {:else}
        {$t("settingsPage.modals.create")}
      {/if}
    </Button>
  {/snippet}
</Modal>

<!-- 删除用户确认弹窗 -->
<Modal bind:open={showDeleteUserConfirm} title={$t("settingsPage.modals.deleteUser")} size="sm" showFooter>
  <div class="form-content">
    {#if deleteTargetUser}
      <Alert variant="warning">
        {$t("settingsPage.modals.deleteUserConfirm", { values: { username: deleteTargetUser.username } })}
      </Alert>
    {/if}
  </div>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (showDeleteUserConfirm = false)}>{$t("settingsPage.modals.cancel")}</Button>
    <Button variant="destructive" onclick={handleDeleteUser}>{$t("settingsPage.modals.delete")}</Button>
  {/snippet}
</Modal>

<!-- 修改密码弹窗 -->
<Modal bind:open={showResetPasswordModal} title={$t("settingsPage.modals.resetPassword")} size="sm" showFooter>
  <div class="form-content">
    {#if resetPasswordTargetUser}
      <p style="margin-bottom: 16px; color: var(--text-secondary);">{$t("settingsPage.modals.resetPasswordFor", { values: { username: resetPasswordTargetUser.username } })}</p>
    {/if}
    <div class="form-group">
      <label>{$t("settingsPage.modals.newPassword")}</label>
      <Input bind:value={resetNewPassword} type="password" placeholder={$t("settingsPage.modals.passwordHint")} />
    </div>
  </div>
  {#snippet footer()}
    <Button variant="ghost" onclick={() => (showResetPasswordModal = false)}>{$t("settingsPage.modals.cancel")}</Button>
    <Button variant="primary" onclick={handleResetPassword} disabled={!resetNewPassword}>{$t("settingsPage.modals.confirmReset")}</Button>
  {/snippet}
</Modal>

<style>
  .settings {
    display: flex;
    height: 100%;
    background: var(--bg-secondary, #f5f5f5);
  }

  .sidebar {
    width: 200px;
    background: var(--bg-card, white);
    border-right: 1px solid var(--border-color, #e0e0e0);
    padding: 8px;
    flex-shrink: 0;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    cursor: pointer;
    font-size: 14px;
    color: var(--text-primary, #333);
    text-align: left;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f5f5f5);
    }

    &.active {
      background: rgba(74, 144, 217, 0.1);
      color: var(--color-primary, #4a90d9);
    }
  }

  .content {
    flex: 1;
    overflow: auto;
    padding: 24px;
  }

  .section {
    max-width: 640px;

    h2 {
      font-size: 24px;
      font-weight: 600;
      margin-bottom: 20px;
      color: var(--text-primary, #333);
    }
  }

  .card {
    background: var(--bg-card, white);
    border-radius: 12px;
    padding: 20px;
    margin-bottom: 16px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.06);

    h3 {
      font-size: 15px;
      font-weight: 600;
      margin: 0 0 16px;
      color: var(--text-primary, #333);
    }
  }

  .user-info {
    display: flex;
    align-items: center;
    gap: 16px;

    .avatar {
      color: var(--color-primary, #4a90d9);
      position: relative;
      width: 64px;
      height: 64px;
      border-radius: 50%;
      overflow: hidden;
      cursor: pointer;
      flex-shrink: 0;

      .avatar-image {
        width: 100%;
        height: 100%;
        object-fit: cover;
        border-radius: 50%;
      }

      .avatar-overlay {
        position: absolute;
        inset: 0;
        background: rgba(0, 0, 0, 0.4);
        display: flex;
        align-items: center;
        justify-content: center;
        opacity: 0;
        transition: opacity 0.2s;
        color: white;
      }

      &:hover .avatar-overlay {
        opacity: 1;
      }
    }

    .info {
      flex: 1;

      h3 {
        margin: 0;
        font-size: 18px;
      }

      p {
        margin: 4px 0 0;
        color: var(--text-muted, #666);
        font-size: 14px;
      }
    }

    .avatar-image {
      width: 64px;
      height: 64px;
      border-radius: 50%;
      object-fit: cover;
    }
  }

  /* 基本信息只读显示 */
  .info-display {
    .info-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 12px 0;
      border-bottom: 1px solid var(--border-color, #f0f0f0);

      &:last-child {
        border-bottom: none;
        padding-bottom: 0;
      }

      &:first-child {
        padding-top: 0;
      }
    }

    .info-label {
      font-size: 14px;
      color: var(--text-muted, #666);
    }

    .info-value {
      font-size: 14px;
      color: var(--text-primary, #333);
    }
  }

  .form-group {
    margin-bottom: 16px;

    label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      margin-bottom: 6px;
      color: var(--text-secondary, #666);
    }
  }

  .form-actions {
    display: flex;
    gap: 12px;
    justify-content: flex-end;
    padding-top: 8px;
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--border-color, #f0f0f0);

    &:last-child {
      border-bottom: none;
      padding-bottom: 0;
    }

    &:first-child {
      padding-top: 0;
    }

    strong {
      display: block;
      font-size: 14px;
      margin-bottom: 4px;
      color: var(--text-primary, #333);
    }

    p {
      margin: 0;
      font-size: 13px;
      color: var(--text-muted, #888);
    }
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    padding: 10px 0;
    font-size: 14px;
    border-bottom: 1px solid var(--border-color, #f0f0f0);

    &:last-child {
      border-bottom: none;
    }

    span:first-child {
      color: var(--text-muted, #666);
    }

    span:last-child {
      color: var(--text-primary, #333);
      font-weight: 500;
    }
  }

  .status-badge {
    padding: 4px 10px;
    border-radius: 12px;
    font-size: 12px;
    font-weight: 500;

    &.success {
      background: #f6ffed;
      color: #52c41a;
    }
  }

  .theme-options {
    display: flex;
    gap: 16px;
  }

  .theme-option {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 12px;
    background: none;
    border: 2px solid transparent;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f5f5f5);
    }

    &.active {
      border-color: var(--color-primary, #4a90d9);
    }

    .preview {
      width: 80px;
      height: 50px;
      border-radius: 6px;
      border: 1px solid var(--border-color, #e0e0e0);

      &.light {
        background: linear-gradient(180deg, #f5f5f5 0%, white 100%);
      }

      &.dark {
        background: linear-gradient(180deg, #1a1a1a 0%, #2d2d2d 100%);
      }

      &.auto {
        background: linear-gradient(135deg, white 50%, #2d2d2d 50%);
      }
    }

    span {
      font-size: 13px;
      color: var(--text-primary, #333);
    }
  }

  .color-options {
    display: flex;
    gap: 12px;
  }

  .color-option {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: 2px solid transparent;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      transform: scale(1.1);
    }

    &.active {
      border-color: var(--text-primary, #333);
      box-shadow: 0 0 0 2px white inset;
    }
  }

  .wallpaper-tabs {
    display: flex;
    gap: 8px;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    padding-bottom: 8px;
  }

  .wallpaper-tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    background: none;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
    color: var(--text-muted, #888);
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f5f5f5);
      color: var(--text-primary, #333);
    }

    &.active {
      background: var(--color-primary, #4a90d9);
      color: white;
    }
  }

  .wallpaper-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .wallpaper-option {
    aspect-ratio: 16 / 10;
    border: 2px solid transparent;
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    background: var(--bg-tertiary, #f0f0f0);
    transition: all 0.15s;
    position: relative;

    img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }

    .wallpaper-name {
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      padding: 4px 8px;
      background: linear-gradient(transparent, rgba(0, 0, 0, 0.7));
      color: white;
      font-size: 11px;
      text-align: center;
      opacity: 0;
      transition: opacity 0.15s;
    }

    &:hover {
      border-color: var(--border-color, #d0d0d0);

      .wallpaper-name {
        opacity: 1;
      }
    }

    &.active {
      border-color: var(--color-primary, #4a90d9);
      box-shadow: 0 0 0 2px rgba(74, 144, 217, 0.3);
    }

    &.upload {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 4px;
      color: var(--text-muted, #888);

      span {
        font-size: 12px;
      }

      &:hover {
        background: var(--bg-hover, #e8e8e8);
      }
    }
  }

  .wallpaper-info-hint {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-muted, #888);
    margin-bottom: 12px;
    padding: 8px 12px;
    background: var(--bg-tertiary, #f5f5f5);
    border-radius: 6px;
  }

  .effect-list {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .effect-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    border: 1px solid var(--border-color, #d0d0d0);
    border-radius: 8px;
    background: var(--bg-primary, white);
    cursor: pointer;
    transition: all 0.15s;
    color: var(--text-primary, #333);
    font-size: 14px;

    &:hover {
      background: var(--bg-hover, #f5f5f5);
      border-color: var(--accent-color, #007aff);
    }

    &.active {
      background: var(--accent-color, #007aff);
      border-color: var(--accent-color, #007aff);
      color: white;
    }
  }

  .loading-placeholder {
    grid-column: 1 / -1;
    text-align: center;
    padding: 40px;
    color: var(--text-muted, #888);
  }

  .custom-wallpaper-form {
    .form-divider {
      text-align: center;
      color: var(--text-muted, #888);
      margin: 16px 0;
      position: relative;

      &::before,
      &::after {
        content: "";
        position: absolute;
        top: 50%;
        width: 40%;
        height: 1px;
        background: var(--border-color, #e0e0e0);
      }

      &::before {
        left: 0;
      }

      &::after {
        right: 0;
      }
    }

    .file-input {
      width: 100%;
      padding: 8px;
      border: 1px dashed var(--border-color, #d0d0d0);
      border-radius: 8px;
      cursor: pointer;

      &:hover {
        border-color: var(--color-primary, #4a90d9);
      }
    }

    .file-preview {
      margin-top: 8px;
      padding: 8px 12px;
      background: var(--bg-tertiary, #f5f5f5);
      border-radius: 6px;
      font-size: 13px;
      color: var(--text-secondary, #666);
    }
  }

  .disk-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }

  .disk-info {
    display: flex;
    align-items: center;
    gap: 12px;
    color: var(--text-muted, #666);

    h3 {
      margin: 0;
      font-size: 14px;
    }

    .disk-path {
      font-size: 12px;
      color: var(--text-muted, #888);
    }
  }

  .disk-usage {
    font-size: 14px;
    font-weight: 500;
    color: var(--text-primary, #333);
  }

  .power-actions {
    display: flex;
    gap: 12px;
  }

  .about-info {
    text-align: center;
    padding: 20px 0;

    .logo-wrapper {
      color: var(--color-primary, #4a90d9);
      margin-bottom: 16px;
    }

    h3 {
      margin: 0 0 8px;
      font-size: 24px;
      text-align: center;
    }

    .version {
      color: var(--text-muted, #666);
      font-size: 14px;
      margin: 0 0 8px;
    }

    .desc {
      color: var(--text-secondary, #888);
      font-size: 14px;
      margin: 0;
    }
  }

  /* 单选组样式 */
  .radio-group {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;

    &.vertical {
      flex-direction: column;
    }
  }

  .radio-option {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    cursor: pointer;
    padding: 8px 12px;
    border: 1px solid var(--border-color, #e5e5e5);
    border-radius: 6px;
    transition: all 0.15s ease;

    &:hover {
      background: var(--bg-hover, rgba(0, 0, 0, 0.03));
    }

    &:has(input:checked) {
      border-color: var(--color-primary, #4a90d9);
      background: rgba(74, 144, 217, 0.05);
    }

    input {
      accent-color: var(--color-primary, #4a90d9);
      margin-top: 2px;
    }

    span {
      font-size: 14px;
    }

    .radio-content {
      display: flex;
      flex-direction: column;
      gap: 2px;

      span {
        font-size: 14px;
        font-weight: 500;
      }
    }

    .radio-hint {
      font-size: 12px;
      color: var(--text-secondary, #666);
      margin: 0;
    }
  }

  /* 泡泡通知状态 */
  .bubble-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--text-secondary, #666);
    padding: 8px 12px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 6px;
  }

  /* 表单行布局 */
  .form-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  /* 危险操作卡片 */
  .danger-card {
    border: 1px solid #fecaca;
    background: #fef2f2;

    h3 {
      color: #dc2626;
    }
  }

  /* 恢复出厂设置弹窗内容 */
  .factory-reset-content {
    text-align: center;
    padding: 12px 0;

    .warning-icon {
      font-size: 48px;
      margin-bottom: 16px;
    }

    .warning-text {
      font-size: 15px;
      color: var(--text-secondary, #666);
      margin-bottom: 20px;
      line-height: 1.6;
    }

    .warning-list {
      background: #fef2f2;
      border: 1px solid #fecaca;
      border-radius: 8px;
      padding: 16px;
      text-align: left;
      margin-bottom: 20px;

      h4 {
        font-size: 14px;
        font-weight: 600;
        color: #dc2626;
        margin: 0 0 12px 0;
      }

      ul {
        margin: 0;
        padding-left: 20px;
        color: #7f1d1d;
        font-size: 14px;

        li {
          margin-bottom: 6px;
        }
      }
    }

    .reset-options {
      text-align: left;

      h4 {
        font-size: 14px;
        font-weight: 500;
        color: var(--text-primary, #333);
        margin: 0 0 12px 0;
      }

      .option-item {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 8px 12px;
        border: 1px solid var(--border-color, #e5e5e5);
        border-radius: 6px;
        cursor: pointer;
        margin-bottom: 8px;
        transition: all 0.15s ease;

        &:hover {
          border-color: var(--color-primary, #4a90d9);
          background: rgba(74, 144, 217, 0.03);
        }

        input[type="checkbox"] {
          width: 16px;
          height: 16px;
          accent-color: var(--color-primary, #4a90d9);
        }

        span {
          font-size: 14px;
          color: var(--text-primary, #333);
        }
      }
    }

    .form-group {
      text-align: left;
      margin-bottom: 16px;

      label {
        display: block;
        font-size: 14px;
        font-weight: 500;
        margin-bottom: 8px;
        color: var(--text-primary, #333);
      }
    }

    &.processing {
      padding: 40px 20px;

      .spinner {
        width: 48px;
        height: 48px;
        border: 4px solid #fee2e2;
        border-top-color: #dc2626;
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 20px;
      }

      .processing-text {
        font-size: 15px;
        color: var(--text-secondary, #666);
        margin-bottom: 16px;
      }

      .progress-log {
        background: var(--bg-secondary, #f5f5f5);
        border-radius: 8px;
        padding: 16px;
        text-align: left;
        max-height: 120px;
        overflow-y: auto;

        p {
          margin: 4px 0;
          font-size: 13px;
          color: var(--text-secondary, #666);
          font-family: monospace;
        }
      }
    }
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .loading-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px;
    color: var(--text-muted, #888);
    gap: 12px;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px;
    color: var(--text-muted, #888);
    gap: 12px;

    p {
      margin: 0;
    }
  }
  .status-info {
    margin-top: 16px;
    padding-top: 16px;
    border-top: 1px solid var(--border-color, #e5e5e5);
  }

  .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px;
    color: var(--text-muted, #888);
    gap: 16px;
    text-align: center;
  }

  .warning-card {
    background: #fffbeb;
    border: 1px solid #fcd34d;
    border-radius: 12px;
    padding: 16px;

    .warning-header {
      display: flex;
      align-items: center;
      gap: 8px;
      color: #92400e;
      margin-bottom: 12px;
    }

    .warning-content {
      font-size: 13px;
      color: #a16207;
      line-height: 1.6;

      p {
        margin: 0 0 4px 0;

        &:last-child {
          margin-bottom: 0;
        }
      }
    }
  }
  /* 推送渠道相关样式 */
  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 8px;

    h3 {
      margin: 0 !important;
    }
  }

  .card-description {
    font-size: 13px;
    color: var(--text-muted, #666);
    margin: 0 0 16px;
  }

  .channel-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .channel-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px;
    background: var(--bg-secondary, #f8f8f8);
    border-radius: 8px;
    transition: background 0.2s;

    &:hover {
      background: var(--bg-hover, #f0f0f0);
    }
  }

  .channel-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .channel-icon {
    width: 40px;
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 10px;
    flex-shrink: 0;
  }

  .channel-details {
    display: flex;
    flex-direction: column;
    gap: 2px;

    strong {
      font-size: 14px;
      font-weight: 600;
      color: var(--text-primary, #333);
    }

    .channel-type {
      font-size: 12px;
      color: var(--text-muted, #666);
    }
  }

  .channel-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .channel-form {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .channel-type-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 12px;
  }

  .channel-type-option {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px 8px;
    background: var(--bg-secondary, #f8f8f8);
    border: 2px solid transparent;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      background: var(--bg-hover, #f0f0f0);
    }

    &.selected {
      border-color: var(--color-primary, #4a90d9);
      background: rgba(74, 144, 217, 0.05);
    }

    .type-icon {
      width: 48px;
      height: 48px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 12px;
    }

    span {
      font-size: 13px;
      font-weight: 500;
      color: var(--text-primary, #333);
    }
  }

  .config-fields {
    display: flex;
    flex-direction: column;
    gap: 12px;

    .field {
      label {
        display: block;
        font-size: 13px;
        color: var(--text-muted, #666);
        margin-bottom: 6px;
      }
    }
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  .form-hint {
    font-size: 12px;
    color: var(--text-muted, #888);
    margin-top: 4px;
  }

  .proxy-test-result {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 12px;
    padding: 8px 12px;
    border-radius: 6px;
    font-size: 13px;

    &.success {
      background: rgba(76, 175, 80, 0.1);
      color: #4caf50;
    }

    &.error {
      background: rgba(244, 67, 54, 0.1);
      color: #f44336;
    }
  }

  /* 用户管理样式 */
  .users-list {
    display: flex;
    flex-direction: column;
  }

  .user-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 0;
    border-bottom: 1px solid var(--border-color, #e0e0e0);

    &:last-child {
      border-bottom: none;
    }
  }

  .user-row-left {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .user-avatar-sm {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-secondary);

    img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
  }

  .user-info-col {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .user-name-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .user-name {
    font-weight: 500;
    font-size: 14px;
    color: var(--text-primary);
  }

  .online-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #4caf50;
    flex-shrink: 0;
  }

  .user-meta {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .user-row-actions {
    display: flex;
    gap: 4px;
  }

  /* 两步验证设置 */
  .twofa-setup {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .twofa-desc {
    font-size: 14px;
    color: var(--text-secondary, #666);
    line-height: 1.5;
    margin: 0;
  }

  .twofa-qr {
    display: flex;
    justify-content: center;
    padding: 16px;
    background: white;
    border-radius: 12px;
    border: 1px solid var(--border-color, #e0e0e0);

    img {
      border-radius: 4px;
    }
  }

  .twofa-secret {
    label {
      display: block;
      font-size: 13px;
      color: var(--text-secondary, #666);
      margin-bottom: 6px;
    }
    code {
      display: block;
      padding: 10px 14px;
      background: var(--bg-secondary, #f5f5f5);
      border-radius: 8px;
      font-family: monospace;
      font-size: 14px;
      letter-spacing: 0.1em;
      word-break: break-all;
      user-select: all;
    }
  }

  .twofa-verify {
    label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      margin-bottom: 8px;
    }
  }

  .twofa-code-input {
    display: flex;
    gap: 8px;
    align-items: flex-start;
  }

  .twofa-success {
    text-align: center;
    color: #4caf50;
    padding: 8px 0;

    h3 {
      margin: 8px 0 4px;
      color: var(--text-primary, #333);
    }
    p {
      color: var(--text-secondary, #666);
      font-size: 14px;
      margin: 0;
    }
  }

  .twofa-backup {
    h4 {
      margin: 0;
      font-size: 14px;
    }
  }

  .twofa-backup-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
  }

  .twofa-copy-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 4px 10px;
    font-size: 12px;
    color: var(--color-primary, #4a90d9);
    background: transparent;
    border: 1px solid var(--color-primary, #4a90d9);
    border-radius: 6px;
    cursor: pointer;
    transition: background 0.15s;

    &:hover {
      background: rgba(74, 144, 217, 0.08);
    }
  }

  .twofa-backup-warn {
    font-size: 13px;
    color: #e65100;
    margin: 0 0 12px;
  }

  .twofa-backup-codes-block {
    padding: 14px 16px;
    background: var(--bg-secondary, #f5f5f5);
    border-radius: 8px;
    font-family: monospace;
    font-size: 14px;
    line-height: 2;
    letter-spacing: 0.08em;
    user-select: all;
    white-space: pre;
    margin: 0;
    overflow-x: auto;
  }
</style>
