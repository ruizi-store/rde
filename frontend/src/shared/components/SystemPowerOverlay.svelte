<script lang="ts">
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { systemPower } from "$shared/stores/system-power.svelte";

  let title = $derived.by(() => {
    if (systemPower.mode === "shutdown") {
      return systemPower.phase === "offline"
        ? $t("desktop.powerOverlay.shutdownDoneTitle")
        : $t("desktop.powerOverlay.shutdownTitle");
    }
    if (systemPower.phase === "coming_back") {
      return $t("desktop.powerOverlay.rebootReadyTitle");
    }
    if (systemPower.phase === "offline") {
      return $t("desktop.powerOverlay.rebootOfflineTitle");
    }
    return $t("desktop.powerOverlay.rebootTitle");
  });

  let desc = $derived.by(() => {
    if (systemPower.mode === "shutdown") {
      return systemPower.phase === "offline"
        ? $t("desktop.powerOverlay.shutdownDoneDesc")
        : $t("desktop.powerOverlay.shutdownDesc");
    }
    if (systemPower.phase === "coming_back") {
      return $t("desktop.powerOverlay.rebootReadyDesc");
    }
    if (systemPower.phase === "offline") {
      return $t("desktop.powerOverlay.rebootOfflineDesc");
    }
    return $t("desktop.powerOverlay.rebootDesc");
  });

  let icon = $derived(
    systemPower.mode === "shutdown" ? "mdi:power" : "mdi:restart",
  );
</script>

{#if systemPower.active}
  <div class="power-overlay" role="alertdialog" aria-modal="true" aria-live="assertive">
    <div class="panel">
      <div class="icon-wrap" class:shutdown={systemPower.mode === "shutdown"}>
        <Icon {icon} width="48" />
      </div>
      <h1>{title}</h1>
      <p>{desc}</p>
      {#if !(systemPower.mode === "shutdown" && systemPower.phase === "offline")}
        <div class="spinner" aria-hidden="true"></div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .power-overlay {
    position: fixed;
    inset: 0;
    z-index: 2147483000;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(8, 12, 20, 0.92);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    pointer-events: all;
    user-select: none;
    cursor: wait;
  }

  .panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 14px;
    max-width: 420px;
    padding: 32px 28px;
    text-align: center;
    color: #e8eef8;
  }

  .icon-wrap {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 88px;
    height: 88px;
    border-radius: 50%;
    background: rgba(253, 126, 20, 0.15);
    color: #fd7e14;
  }

  .icon-wrap.shutdown {
    background: rgba(220, 53, 69, 0.15);
    color: #ff6b7a;
  }

  h1 {
    margin: 0;
    font-size: 22px;
    font-weight: 600;
    letter-spacing: 0.02em;
  }

  p {
    margin: 0;
    font-size: 14px;
    line-height: 1.55;
    color: rgba(232, 238, 248, 0.72);
  }

  .spinner {
    width: 28px;
    height: 28px;
    margin-top: 8px;
    border: 3px solid rgba(255, 255, 255, 0.15);
    border-top-color: rgba(255, 255, 255, 0.85);
    border-radius: 50%;
    animation: spin 0.9s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
