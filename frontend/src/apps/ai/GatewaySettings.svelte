<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "./i18n";
  import { Button, Input, Spinner, Select, Switch, Tabs, Alert as AlertBox } from "$shared/ui";
  import {
    aiService,
    type GatewayConfig,
    type GatewayStatus,
    type AlertConfig,
    type Alert,
    type AlertStatus,
  } from "./service";

  let { onclose }: { onclose: () => void } = $props();

  let activeTab = $state("gateway");
  let loading = $state(false);
  let error = $state("");
  let success = $state("");

  // Gateway state
  let gwConfig = $state<GatewayConfig | null>(null);
  let gwStatus = $state<GatewayStatus | null>(null);

  // Alert state
  let alertConfig = $state<AlertConfig | null>(null);
  let alertStatus = $state<AlertStatus | null>(null);
  let alerts = $state<Alert[]>([]);

  const tabs = $derived([
    { id: "gateway", label: $t("ai.gateway.messageGateway") },
    { id: "alerts", label: $t("ai.gateway.alertMonitor") },
  ]);

  onMount(async () => {
    await loadGateway();
  });

  async function loadGateway() {
    loading = true;
    error = "";
    try {
      const [c, s] = await Promise.all([aiService.getGatewayConfig(), aiService.getGatewayStatus()]);
      gwConfig = c;
      gwStatus = s;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadAlerts() {
    loading = true;
    error = "";
    try {
      const [c, s, h] = await Promise.all([
        aiService.getAlertsConfig(),
        aiService.getAlertsStatus(),
        aiService.getAlertsHistory(),
      ]);
      alertConfig = c;
      alertStatus = s;
      alerts = h || [];
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function saveGateway() {
    if (!gwConfig) return;
    error = "";
    success = "";
    try {
      await aiService.updateGatewayConfig(gwConfig);
      success = $t("ai.gateway.gatewaySaved");
      setTimeout(() => (success = ""), 2000);
    } catch (e: any) {
      error = e.message;
    }
  }

  async function toggleGateway() {
    try {
      if (gwStatus?.enabled) {
        await aiService.stopGateway();
      } else {
        await aiService.startGateway();
      }
      gwStatus = await aiService.getGatewayStatus();
    } catch (e: any) {
      error = e.message;
    }
  }

  async function saveAlerts() {
    if (!alertConfig) return;
    error = "";
    success = "";
    try {
      await aiService.updateAlertsConfig(alertConfig);
      success = $t("ai.gateway.alertSaved");
      setTimeout(() => (success = ""), 2000);
    } catch (e: any) {
      error = e.message;
    }
  }

  async function toggleAlerts() {
    try {
      if (alertStatus?.running) {
        await aiService.stopAlerts();
      } else {
        await aiService.startAlerts();
      }
      alertStatus = await aiService.getAlertsStatus();
    } catch (e: any) {
      error = e.message;
    }
  }

  async function clearAlerts() {
    try {
      await aiService.clearAlertsHistory();
      alerts = [];
    } catch (e: any) {
      error = e.message;
    }
  }

  function onTabChange(tab: string) {
    activeTab = tab;
    error = "";
    success = "";
    if (tab === "alerts" && !alertConfig) loadAlerts();
    if (tab === "gateway" && !gwConfig) loadGateway();
  }

  function alertLevelColor(level: string): string {
    if (level === "critical") return "text-red-500";
    if (level === "warning") return "text-yellow-500";
    return "text-blue-500";
  }
</script>

<Tabs {tabs} bind:activeTab variant="underline" size="sm" onchange={onTabChange}>
  {#snippet children(tab)}
  <div class="space-y-4 pt-2">
  {#if loading}
    <div class="flex justify-center p-8"><Spinner /></div>
  {:else if tab === "gateway" && gwConfig}
    <!-- Gateway Settings -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.gateway.messageGateway")}</h3>
          <p class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.gatewayDescription")}</p>
        </div>
        <Button size="sm" variant={gwStatus?.enabled ? "danger" : "primary"} onclick={toggleGateway}>
          {gwStatus?.enabled ? $t("ai.gateway.stop") : $t("ai.gateway.start")}
        </Button>
      </div>

      <!-- Telegram -->
      <details class="border border-[var(--border-primary)] rounded-lg" open>
        <summary class="px-4 py-3 cursor-pointer text-sm font-medium text-[var(--text-primary)] bg-[var(--bg-primary)] rounded-t-lg">
          Telegram Bot
          {#if gwConfig.telegram.enabled}
            <span class="ml-2 text-xs px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300">{$t("ai.gateway.enable")}</span>
          {/if}
        </summary>
        <div class="p-4 space-y-3">
          <Switch bind:checked={gwConfig.telegram.enabled} label={$t("ai.gateway.enableTelegram")} />
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">Bot Token</label>
            <Input bind:value={gwConfig.telegram.bot_token} placeholder="123456:ABC..." type="password" />
          </div>
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.proxyMode")}</label>
            <Select options={[{value:"off",label:$t("ai.gateway.proxyOff")},{value:"system",label:$t("ai.gateway.proxySystem")},{value:"custom",label:$t("ai.gateway.proxyCustom")}]} bind:value={gwConfig.telegram.proxy_mode} />
          </div>
          {#if gwConfig.telegram.proxy_mode === "custom"}
            <div>
              <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.proxyAddress")}</label>
              <Input bind:value={gwConfig.telegram.proxy_url} placeholder="socks5://127.0.0.1:1080" />
            </div>
          {/if}
        </div>
      </details>

      <!-- Wecom -->
      <details class="border border-[var(--border-primary)] rounded-lg">
        <summary class="px-4 py-3 cursor-pointer text-sm font-medium text-[var(--text-primary)] bg-[var(--bg-primary)] rounded-t-lg">
          企业微信
          {#if gwConfig.wecom.enabled}
            <span class="ml-2 text-xs px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300">{$t("ai.gateway.enable")}</span>
          {/if}
        </summary>
        <div class="p-4 space-y-3">
          <Switch bind:checked={gwConfig.wecom.enabled} label={$t("ai.gateway.enableWecom")} />
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">Corp ID</label>
            <Input bind:value={gwConfig.wecom.corp_id} placeholder="ww..." />
          </div>
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">Secret</label>
            <Input bind:value={gwConfig.wecom.secret} type="password" />
          </div>
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">Token</label>
            <Input bind:value={gwConfig.wecom.token} />
          </div>
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">EncodingAESKey</label>
            <Input bind:value={gwConfig.wecom.encoding_key} />
          </div>
        </div>
      </details>

      <!-- Webhook -->
      <details class="border border-[var(--border-primary)] rounded-lg">
        <summary class="px-4 py-3 cursor-pointer text-sm font-medium text-[var(--text-primary)] bg-[var(--bg-primary)] rounded-t-lg">
          Webhook
          {#if gwConfig.webhook.enabled}
            <span class="ml-2 text-xs px-1.5 py-0.5 rounded bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300">{$t("ai.gateway.enable")}</span>
          {/if}
        </summary>
        <div class="p-4 space-y-3">
          <Switch bind:checked={gwConfig.webhook.enabled} label={$t("ai.gateway.enableWebhook")} />
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">API Key</label>
            <Input bind:value={gwConfig.webhook.api_key} type="password" />
          </div>
        </div>
      </details>

      <!-- Security -->
      <details class="border border-[var(--border-primary)] rounded-lg">
        <summary class="px-4 py-3 cursor-pointer text-sm font-medium text-[var(--text-primary)] bg-[var(--bg-primary)] rounded-t-lg">{$t("ai.gateway.securitySettings")}</summary>
        <div class="p-4 space-y-3">
          <Switch bind:checked={gwConfig.security.require_pin} label={$t("ai.gateway.requirePin")} />
          {#if gwConfig.security.require_pin}
            <div>
              <label class="text-xs text-[var(--text-tertiary)]">PIN</label>
              <Input bind:value={gwConfig.security.pin} type="password" placeholder={$t("ai.gateway.pinNumber")} />
            </div>
          {/if}
          <div>
            <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.dailyLimit")}</label>
            <input type="number" class="num-input" bind:value={gwConfig.security.daily_limit} />
          </div>
        </div>
      </details>

      <div class="flex justify-end">
        <Button variant="primary" onclick={saveGateway}>{$t("ai.gateway.saveConfig")}</Button>
      </div>
    </div>

  {:else if tab === "alerts" && alertConfig}
    <!-- Alerts Settings -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.gateway.alertMonitor")}</h3>
          <p class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.monitorDescription")}</p>
        </div>
        <Button size="sm" variant={alertStatus?.running ? "danger" : "primary"} onclick={() => toggleAlerts()}>
          {alertStatus?.running ? $t("ai.gateway.stop") : $t("ai.gateway.start")}
        </Button>
      </div>

      <!-- Config -->
      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.checkInterval")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.check_interval} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.cooldownMinutes")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.cooldown_minutes} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.diskWarning")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.disk_warning_pct} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.diskCritical")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.disk_critical_pct} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.cpuWarning")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.cpu_warning_pct} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.memoryWarning")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.memory_warning_pct} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.tempWarning")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.temp_warning_c} />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.quietStart")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.quiet_hours_start} />
        </div>
        <div>
          <label class="text-xs text-[var(--text-tertiary)]">{$t("ai.gateway.quietEnd")}</label>
          <input type="number" class="num-input" bind:value={alertConfig.quiet_hours_end} />
        </div>
      </div>

      <div class="flex justify-end">
        <Button variant="primary" onclick={saveAlerts}>{$t("ai.gateway.saveConfig")}</Button>
      </div>

      <!-- Alert History -->
      {#if alerts.length > 0}
        <div class="space-y-2">
          <div class="flex items-center justify-between">
            <h4 class="text-sm font-medium text-[var(--text-primary)]">{$t("ai.gateway.alertHistory")} ({alerts.length})</h4>
            <Button size="sm" variant="ghost" onclick={clearAlerts}>{$t("ai.gateway.clear")}</Button>
          </div>
          <div class="max-h-48 overflow-y-auto space-y-1">
            {#each alerts as alert}
              <div class="flex items-start gap-2 p-2 rounded bg-[var(--bg-primary)] text-sm">
                <span class="font-medium {alertLevelColor(alert.level)}">
                  {alert.level === "critical" ? "🔴" : alert.level === "warning" ? "🟡" : "🔵"}
                </span>
                <div class="flex-1 min-w-0">
                  <div class="font-medium text-[var(--text-primary)]">{alert.title}</div>
                  <div class="text-xs text-[var(--text-tertiary)]">{alert.message}</div>
                </div>
                <span class="text-xs text-[var(--text-tertiary)] whitespace-nowrap">
                  {new Date(alert.timestamp).toLocaleString("zh-CN")}
                </span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  {/if}

  {#if error}
    <AlertBox type="error">{error}</AlertBox>
  {/if}
  {#if success}
    <AlertBox type="success">{success}</AlertBox>
  {/if}
  </div>
  {/snippet}
</Tabs>

<style>
  .num-input {
    width: 100%;
    font-size: 0.875rem;
    background: var(--bg-primary);
    color: var(--text-primary);
    border: 1px solid var(--border-primary);
    border-radius: 0.375rem;
    padding: 0.375rem 0.75rem;
  }
  .num-input:focus {
    outline: none;
    border-color: #3b82f6;
    box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.15);
  }
</style>
