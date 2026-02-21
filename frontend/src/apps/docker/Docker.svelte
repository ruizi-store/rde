<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { Button, Modal, Spinner, EmptyState, Tabs, Badge } from "$shared/ui";
  import { showToast } from "$shared/ui";
  import {
    dockerService,
    type DockerContainer,
    type DockerImage,
    type DockerNetwork,
    type DockerInfo,
    type CreateContainerRequest,
  } from "./service";
  import { type StoreAppDetail, dockerStoreService } from "./store-service";
  import DockerStore from "./DockerStore.svelte";
  import DockerAppDetail from "./DockerAppDetail.svelte";
  import DockerInstall from "./DockerInstall.svelte";
  import DockerMyApps from "./DockerMyApps.svelte";
  import { refreshExternalApps } from "$apps";

  // ==================== 状态 ====================

  let containers = $state<DockerContainer[]>([]);
  let images = $state<DockerImage[]>([]);
  let networks = $state<DockerNetwork[]>([]);
  let info = $state<DockerInfo | null>(null);
  let loading = $state(true);
  let available = $state(true);
  let activeTab = $state("store");
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  // 创建容器
  let showCreate = $state(false);
  let createName = $state("");
  let createImage = $state("");
  let createPorts = $state("");
  let createVolumes = $state("");
  let createEnv = $state("");
  let createRestart = $state("no");

  // 拉取镜像
  let showPull = $state(false);
  let pullImageName = $state("");
  let pulling = $state(false);

  // 日志
  let showLogs = $state(false);
  let logsTitle = $state("");
  let logsContent = $state("");

  // 创建网络
  let showCreateNetwork = $state(false);
  let networkName = $state("");
  let networkDriver = $state("bridge");

  let tabs = $derived([
    { id: "store", label: $t("docker.tabs.store") },
    { id: "myapps", label: $t("docker.tabs.myapps") },
    { id: "containers", label: $t("docker.tabs.containers") },
    { id: "images", label: $t("docker.tabs.images") },
    { id: "networks", label: $t("docker.tabs.networks") },
    { id: "info", label: $t("docker.tabs.info") },
  ]);

  // 应用详情 & 安装
  let selectedAppId = $state("");
  let installApp = $state<StoreAppDetail | null>(null);
  let showInstall = $state(false);
  let installedAppIds = $state<Set<string>>(new Set());

  // 视图模式: "browse" | "detail"
  let storeView = $state<"browse" | "detail">("browse");

  function handleSelectApp(appId: string) {
    selectedAppId = appId;
    storeView = "detail";
  }

  function handleBackToStore() {
    selectedAppId = "";
    storeView = "browse";
  }

  function handleStartInstall(app: StoreAppDetail) {
    installApp = app;
    showInstall = true;
  }

  function handleInstalled() {
    // 安装完成后切换到"我的应用"标签
    storeView = "browse";
    selectedAppId = "";
    activeTab = "myapps";
    showToast($t("docker.appInstallSuccess"), "success");
    refreshContainers();
    refreshInstalledIds();
    // 刷新外部应用列表（更新开始菜单和最近使用）
    refreshExternalApps();
  }

  function handleGoToStore() {
    activeTab = "store";
    storeView = "browse";
  }

  async function refreshInstalledIds() {
    try {
      const apps = await dockerStoreService.getInstalledApps();
      installedAppIds = new Set(apps.map((a) => a.app_id));
    } catch {
      // silently ignore
    }
  }

  // ==================== 生命周期 ====================

  onMount(async () => {
    available = await dockerService.isAvailable();
    if (available) await refresh();
    loading = false;
    if (available) {
      refreshTimer = setInterval(refreshContainers, 8000);
      refreshInstalledIds();
    }
  });

  onDestroy(() => { if (refreshTimer) clearInterval(refreshTimer); });

  // ==================== 方法 ====================

  async function refresh() {
    try {
      const [c, i, n, inf] = await Promise.all([
        dockerService.getContainers(),
        dockerService.getImages(),
        dockerService.getNetworks(),
        dockerService.getInfo(),
      ]);
      containers = c;
      images = i;
      networks = n;
      info = inf;
    } catch {}
  }

  async function refreshContainers() {
    try { containers = await dockerService.getContainers(); } catch {}
  }

  // 容器操作
  async function startContainer(id: string) {
    try { await dockerService.startContainer(id); showToast($t("docker.containerStarted"), "success"); await refreshContainers(); }
    catch (e: any) { showToast(e.message, "error"); }
  }
  async function stopContainer(id: string) {
    try { await dockerService.stopContainer(id); showToast($t("docker.containerStopped"), "success"); await refreshContainers(); }
    catch (e: any) { showToast(e.message, "error"); }
  }
  async function restartContainer(id: string) {
    try { await dockerService.restartContainer(id); showToast($t("docker.containerRestarting"), "success"); await refreshContainers(); }
    catch (e: any) { showToast(e.message, "error"); }
  }
  async function removeContainer(id: string) {
    try { await dockerService.removeContainer(id, true); showToast($t("docker.containerDeleted"), "success"); await refreshContainers(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  async function viewLogs(c: DockerContainer) {
    try {
      logsTitle = c.name;
      logsContent = $t("docker.loading");
      showLogs = true;
      logsContent = await dockerService.getContainerLogs(c.id);
    } catch (e: any) { logsContent = `Error: ${e.message}`; }
  }

  async function createContainer() {
    if (!createName || !createImage) return;
    const config: CreateContainerRequest = { name: createName, image: createImage };
    if (createPorts) {
      config.ports = {};
      createPorts.split(",").forEach((p) => {
        const [host, container] = p.trim().split(":");
        if (host && container) config.ports![container] = host;
      });
    }
    if (createVolumes) {
      config.volumes = {};
      createVolumes.split(",").forEach((v) => {
        const [host, container] = v.trim().split(":");
        if (host && container) config.volumes![container] = host;
      });
    }
    if (createEnv) {
      config.environment = createEnv.split("\n").map((e) => e.trim()).filter(Boolean);
    }
    config.restart = createRestart;
    try {
      await dockerService.createContainer(config);
      showCreate = false;
      createName = ""; createImage = ""; createPorts = ""; createVolumes = ""; createEnv = ""; createRestart = "no";
      showToast($t("docker.containerCreated"), "success");
      await refreshContainers();
    } catch (e: any) { showToast($t("docker.createFailed") + " " + e.message, "error"); }
  }

  // 镜像操作
  async function pullImage() {
    if (!pullImageName) return;
    pulling = true;
    try {
      await dockerService.pullImage(pullImageName);
      showPull = false;
      pullImageName = "";
      showToast($t("docker.imagePulled"), "success");
      images = await dockerService.getImages();
    } catch (e: any) { showToast($t("docker.pullFailed") + " " + e.message, "error"); }
    finally { pulling = false; }
  }

  async function removeImage(id: string) {
    try { await dockerService.removeImage(id, true); showToast($t("docker.imageDeleted"), "success"); images = await dockerService.getImages(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  // 网络操作
  async function createNetwork() {
    if (!networkName) return;
    try {
      await dockerService.createNetwork({ name: networkName, driver: networkDriver });
      showCreateNetwork = false;
      networkName = "";
      showToast($t("docker.networkCreated"), "success");
      networks = await dockerService.getNetworks();
    } catch (e: any) { showToast(e.message, "error"); }
  }

  async function removeNetwork(id: string) {
    try { await dockerService.removeNetwork(id); showToast($t("docker.networkDeleted"), "success"); networks = await dockerService.getNetworks(); }
    catch (e: any) { showToast(e.message, "error"); }
  }

  // 格式化
  function stateColor(s: string): "default" | "success" | "warning" | "error" {
    switch (s) { case "running": return "success"; case "paused": return "warning"; case "restarting": return "warning"; default: return "default"; }
  }
  function stateText(s: string): string {
    const m: Record<string, string> = { running: $t("docker.state.running"), stopped: $t("docker.state.stopped"), exited: $t("docker.state.exited"), created: $t("docker.state.created"), paused: $t("docker.state.paused"), restarting: $t("docker.state.restarting") };
    return m[s] || s;
  }
  function formatBytes(b: number): string {
    if (b < 1024) return b + " B";
    if (b < 1048576) return (b / 1024).toFixed(1) + " KB";
    if (b < 1073741824) return (b / 1048576).toFixed(1) + " MB";
    return (b / 1073741824).toFixed(2) + " GB";
  }
  function shortId(id: string): string { return id.slice(0, 12); }
  function getPortDisplay(c: DockerContainer): string {
    if (!c.ports?.length) return "";
    return c.ports
      .filter((p) => (p.public_port || p.host_port))
      .map((p) => `${p.public_port || p.host_port}:${p.private_port || p.container_port}`)
      .join(", ");
  }
</script>

<div class="docker-mgr">
  {#if loading}
    <Spinner center />
  {:else if !available}
    <EmptyState icon="mdi:docker" title={$t("docker.notRunning")} description={$t("docker.ensureDockerInstalled")} />
  {:else}
    <div class="content">
      <Tabs {tabs} bind:activeTab variant="underline" size="sm">
        {#snippet children(tab)}
          <!-- 应用商店 -->
          {#if tab === "store"}
            {#if storeView === "detail" && selectedAppId}
              <DockerAppDetail
                appId={selectedAppId}
                onBack={handleBackToStore}
                onInstall={handleStartInstall}
                isInstalled={installedAppIds.has(selectedAppId)}
              />
            {:else}
              <DockerStore onSelectApp={handleSelectApp} {installedAppIds} />
            {/if}

          <!-- 我的应用 -->
          {:else if tab === "myapps"}
            <DockerMyApps onBrowseStore={handleGoToStore} />

          <!-- 容器 -->
          {:else if tab === "containers"}
            <div class="panel-header">
              <span class="count">{$t("docker.containerCount", { values: { n: containers.length } })}</span>
              <Button variant="primary" size="sm" onclick={() => (showCreate = true)}>
                {$t("docker.create")}
              </Button>
            </div>
            {#if containers.length === 0}
              <EmptyState icon="mdi:package-variant" title={$t("docker.noContainers")} />
            {:else}
              <div class="item-list">
                {#each containers as c (c.id)}
                  <div class="item-card">
                    <div class="item-top">
                      <div>
                        <div class="item-title">{c.name.replace(/^\//, "")}</div>
                        <div class="item-sub">{c.image} · {shortId(c.id)}</div>
                      </div>
                      <Badge variant={stateColor(c.state)}>{stateText(c.state)}</Badge>
                    </div>
                    {#if getPortDisplay(c)}
                      <div class="item-ports">{$t("docker.port")} {getPortDisplay(c)}</div>
                    {/if}
                    <div class="item-actions">
                      {#if c.state === "running"}
                        <Button variant="ghost" size="sm" onclick={() => stopContainer(c.id)}>{$t("docker.stop")}</Button>
                        <Button variant="ghost" size="sm" onclick={() => restartContainer(c.id)}>{$t("docker.restart")}</Button>
                      {:else}
                        <Button variant="ghost" size="sm" onclick={() => startContainer(c.id)}>{$t("docker.start")}</Button>
                      {/if}
                      <Button variant="ghost" size="sm" onclick={() => viewLogs(c)}>
                        {$t("docker.logs")}
                      </Button>
                      <Button variant="ghost" size="sm" onclick={() => removeContainer(c.id)}>
                        <Icon icon="mdi:delete-outline" width="16" />
                      </Button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}

          <!-- 镜像 -->
          {:else if tab === "images"}
            <div class="panel-header">
              <span class="count">{$t("docker.imageCount", { values: { n: images.length } })}</span>
              <Button variant="primary" size="sm" onclick={() => (showPull = true)}>
                {$t("docker.pull")}
              </Button>
            </div>
            {#if images.length === 0}
              <EmptyState icon="mdi:layers-outline" title={$t("docker.noImages")} />
            {:else}
              <div class="item-list">
                {#each images as img (img.id)}
                  <div class="item-card row">
                    <div>
                      <div class="item-title">{img.tags?.join(", ") || img.repository || shortId(img.id)}</div>
                      <div class="item-sub">{formatBytes(img.size)} · {shortId(img.id)}</div>
                    </div>
                    <Button variant="ghost" size="sm" onclick={() => removeImage(img.id)}>
                      <Icon icon="mdi:delete-outline" width="16" />
                    </Button>
                  </div>
                {/each}
              </div>
            {/if}

          <!-- 网络 -->
          {:else if tab === "networks"}
            <div class="panel-header">
              <span class="count">{$t("docker.networkCount", { values: { n: networks.length } })}</span>
              <Button variant="primary" size="sm" onclick={() => (showCreateNetwork = true)}>
                {$t("docker.create")}
              </Button>
            </div>
            {#if networks.length === 0}
              <EmptyState icon="mdi:lan" title={$t("docker.noNetworks")} />
            {:else}
              <div class="item-list">
                {#each networks as net (net.id)}
                  <div class="item-card row">
                    <div>
                      <div class="item-title">{net.name}</div>
                      <div class="item-sub">{net.driver} · {net.scope}{net.subnet ? ` · ${net.subnet}` : ""}</div>
                    </div>
                    {#if net.name !== "bridge" && net.name !== "host" && net.name !== "none"}
                      <Button variant="ghost" size="sm" onclick={() => removeNetwork(net.id)}>
                        <Icon icon="mdi:delete-outline" width="16" />
                      </Button>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}

          <!-- 系统信息 -->
          {:else if tab === "info"}
            {#if info}
              <div class="info-grid">
                <div class="info-item"><span class="info-label">{$t("docker.infoLabels.version")}</span><span class="info-value">{info.version}</span></div>
                {#if info.api_version}<div class="info-item"><span class="info-label">{$t("docker.infoLabels.apiVersion")}</span><span class="info-value">{info.api_version}</span></div>{/if}
                {#if info.os}<div class="info-item"><span class="info-label">{$t("docker.infoLabels.system")}</span><span class="info-value">{info.os}/{info.arch}</span></div>{/if}
                <div class="info-item"><span class="info-label">{$t("docker.infoLabels.containers")}</span><span class="info-value">{info.containers} ({$t("docker.infoLabels.running")} {info.running ?? 0}, {$t("docker.infoLabels.stopped")} {info.stopped ?? 0})</span></div>
                <div class="info-item"><span class="info-label">{$t("docker.infoLabels.images")}</span><span class="info-value">{info.images}</span></div>
                {#if info.ncpu}<div class="info-item"><span class="info-label">{$t("docker.infoLabels.cpu")}</span><span class="info-value">{info.ncpu} 核</span></div>{/if}
                {#if info.mem_total}<div class="info-item"><span class="info-label">{$t("docker.infoLabels.memory")}</span><span class="info-value">{formatBytes(info.mem_total)}</span></div>{/if}
              </div>
            {/if}
          {/if}
        {/snippet}
      </Tabs>
    </div>
  {/if}
</div>

<!-- 创建容器 -->
<Modal bind:open={showCreate} title={$t("docker.createContainer")} size="md">
  <form class="modal-form" onsubmit={(e) => { e.preventDefault(); createContainer(); }}>
    <div class="form-group">
      <label for="ct-name">{$t("docker.containerName")}</label>
      <input id="ct-name" bind:value={createName} required placeholder="my-container" />
    </div>
    <div class="form-group">
      <label for="ct-image">{$t("docker.image")}</label>
      <input id="ct-image" bind:value={createImage} required placeholder="nginx:latest" />
    </div>
    <div class="form-group">
      <label for="ct-ports">{$t("docker.portMapping")}</label>
      <input id="ct-ports" bind:value={createPorts} placeholder="8080:80, 8443:443" />
    </div>
    <div class="form-group">
      <label for="ct-volumes">{$t("docker.volumeMapping")}</label>
      <input id="ct-volumes" bind:value={createVolumes} placeholder="/data:/app/data" />
    </div>
    <div class="form-group">
      <label for="ct-env">{$t("docker.envVars")}</label>
      <textarea id="ct-env" bind:value={createEnv} rows="3" placeholder="NODE_ENV=production"></textarea>
    </div>
    <div class="form-group">
      <label for="ct-restart">{$t("docker.restartPolicy")}</label>
      <select id="ct-restart" bind:value={createRestart}>
        <option value="no">{$t("docker.restartNo")}</option>
        <option value="always">{$t("docker.restartAlways")}</option>
        <option value="on-failure">{$t("docker.restartOnFailure")}</option>
        <option value="unless-stopped">{$t("docker.restartUnlessStopped")}</option>
      </select>
    </div>
    <Button variant="primary" fullWidth onclick={createContainer}>{$t("docker.create")}</Button>
  </form>
</Modal>

<!-- 拉取镜像 -->
<Modal bind:open={showPull} title={$t("docker.pullImage")} size="sm">
  <form class="modal-form" onsubmit={(e) => { e.preventDefault(); pullImage(); }}>
    <div class="form-group">
      <label for="pull-name">{$t("docker.imageName")}</label>
      <input id="pull-name" bind:value={pullImageName} required placeholder="nginx:latest" />
    </div>
    <Button variant="primary" fullWidth onclick={pullImage} disabled={pulling}>
      {pulling ? $t("docker.pulling") : $t("docker.pull")}
    </Button>
  </form>
</Modal>

<!-- 日志 -->
<Modal bind:open={showLogs} title="{$t('docker.logs')} - {logsTitle}" size="lg">
  <pre class="log-output">{logsContent}</pre>
</Modal>

<!-- 创建网络 -->
<Modal bind:open={showCreateNetwork} title={$t("docker.createNetwork")} size="sm">
  <form class="modal-form" onsubmit={(e) => { e.preventDefault(); createNetwork(); }}>
    <div class="form-group">
      <label for="net-name">{$t("docker.networkName")}</label>
      <input id="net-name" bind:value={networkName} required placeholder="my-network" />
    </div>
    <div class="form-group">
      <label for="net-driver">{$t("docker.driver")}</label>
      <select id="net-driver" bind:value={networkDriver}>
        <option value="bridge">bridge</option>
        <option value="host">host</option>
        <option value="overlay">overlay</option>
        <option value="macvlan">macvlan</option>
      </select>
    </div>
    <Button variant="primary" fullWidth onclick={createNetwork}>{$t("docker.create")}</Button>
  </form>
</Modal>

<!-- 安装向导 -->
{#if installApp}
  <DockerInstall
    app={installApp}
    bind:open={showInstall}
    onClose={() => { installApp = null; }}
    onInstalled={handleInstalled}
  />
{/if}

<style>
  .docker-mgr {
    display: flex;
    flex-direction: column;
    height: 100%;
    background: var(--bg-primary, #f5f5f5);
  }

  .content { flex: 1; padding: 16px 20px; overflow-y: auto; }
  .panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
  .count { font-size: 13px; color: var(--text-muted, #999); }

  .item-list { display: flex; flex-direction: column; gap: 8px; }
  .item-card { background: var(--bg-card, white); border-radius: 8px; padding: 14px 16px; }
  .item-card.row { display: flex; justify-content: space-between; align-items: center; }
  .item-top { display: flex; justify-content: space-between; align-items: flex-start; }
  .item-title { font-weight: 500; font-size: 14px; }
  .item-sub { font-size: 12px; color: var(--text-muted, #999); margin-top: 2px; }
  .item-ports { font-size: 12px; color: var(--text-secondary, #666); margin-top: 6px; }
  .item-actions { display: flex; gap: 6px; margin-top: 10px; flex-wrap: wrap; }

  .info-grid {
    background: var(--bg-card, white);
    border-radius: 8px;
    padding: 16px;
  }
  .info-item {
    display: flex;
    justify-content: space-between;
    padding: 10px 0;
    border-bottom: 1px solid var(--border-color, #f0f0f0);
    &:last-child { border-bottom: none; }
  }
  .info-label { font-size: 13px; color: var(--text-secondary, #666); }
  .info-value { font-size: 13px; font-weight: 500; }

  .modal-form { display: flex; flex-direction: column; gap: 14px; }
  .form-group {
    display: flex; flex-direction: column; gap: 6px;
    label { font-size: 13px; font-weight: 500; color: var(--text-secondary, #666); }
    input, select, textarea {
      padding: 10px 12px; border: 1px solid var(--border-color, #e0e0e0); border-radius: 6px;
      font-size: 14px; background: var(--bg-input, white); color: var(--text-primary, #333);
      font-family: inherit;
      &:focus { outline: none; border-color: var(--color-primary, #4a90d9); }
    }
    textarea { resize: vertical; }
  }

  .log-output {
    max-height: 500px; overflow: auto; padding: 16px;
    background: #1a1a2e; color: #eee; border-radius: 6px; font-size: 12px;
    white-space: pre-wrap; word-break: break-all; line-height: 1.6;
  }
</style>
