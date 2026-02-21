<script lang="ts">
  import Icon from "@iconify/svelte";
  import { Button, Input, Switch, Select } from "$shared/ui";
  import { VIDEO_FILTERS, DEFAULT_KEY_MAPPING, DEFAULT_ROM_DIRECTORY } from "./constants";
  import type { RetroGameSettings, KeyMapping } from "./types";
  import { t } from "svelte-i18n";

  // ==================== Props ====================

  interface Props {
    settings: RetroGameSettings;
    onSave: (settings: RetroGameSettings) => void;
    onCancel: () => void;
  }

  let { settings, onSave, onCancel }: Props = $props();

  // ==================== 状态 ====================

  let localSettings = $state<RetroGameSettings>({ ...settings });
  let activeTab = $state<"general" | "controls" | "video">("general");

  // ==================== 方法 ====================

  function handleSave() {
    onSave(localSettings);
  }

  function resetKeyMapping() {
    localSettings.keyMapping = { ...DEFAULT_KEY_MAPPING };
  }

  function resetAll() {
    localSettings = {
      romDirectory: DEFAULT_ROM_DIRECTORY,
      autoSave: true,
      showFps: false,
      audioVolume: 100,
      videoFilter: "none",
      keyMapping: { ...DEFAULT_KEY_MAPPING },
    };
  }

  const keyLabels: Record<keyof KeyMapping, string> = {
    up: "keyUp",
    down: "keyDown",
    left: "keyLeft",
    right: "keyRight",
    a: "A",
    b: "B",
    x: "X",
    y: "Y",
    start: "Start",
    select: "Select",
    l: "L",
    r: "R",
    l2: "L2",
    r2: "R2",
  };

  // Get localized key label
  function getKeyLabel(key: keyof KeyMapping): string {
    const label = keyLabels[key];
    // Direction keys need translation
    if (["keyUp", "keyDown", "keyLeft", "keyRight"].includes(label)) {
      return $t(`retrogame.${label}`);
    }
    return label;
  }
</script>

<div class="game-settings">
  <!-- 选项卡 -->
  <div class="tabs">
    <button class="tab" class:active={activeTab === "general"} onclick={() => (activeTab = "general")}>
      <Icon icon="mdi:cog" width={18} />
      {$t("retrogame.general")}
    </button>
    <button class="tab" class:active={activeTab === "controls"} onclick={() => (activeTab = "controls")}>
      <Icon icon="mdi:gamepad" width={18} />
      {$t("retrogame.controls")}
    </button>
    <button class="tab" class:active={activeTab === "video"} onclick={() => (activeTab = "video")}>
      <Icon icon="mdi:monitor" width={18} />
      {$t("retrogame.video")}
    </button>
  </div>

  <!-- 内容 -->
  <div class="content">
    {#if activeTab === "general"}
      <div class="section">
        <h3>{$t("retrogame.romDirectory")}</h3>
        <div class="rom-dir-display">
          <span class="dir-path">{localSettings.romDirectory || DEFAULT_ROM_DIRECTORY}</span>
        </div>
        <p class="hint">{$t("retrogame.changeRomDirHint")}</p>
      </div>

      <div class="section">
        <h3>{$t("retrogame.gameOptions")}</h3>
        <label class="option">
          <span>{$t("retrogame.autoSave")}</span>
          <Switch bind:checked={localSettings.autoSave} />
        </label>
        <p class="hint">{$t("retrogame.autoSaveHint")}</p>

        <label class="option">
          <span>{$t("retrogame.showFps")}</span>
          <Switch bind:checked={localSettings.showFps} />
        </label>
      </div>

      <div class="section">
        <h3>{$t("retrogame.audio")}</h3>
        <label class="option">
          <span>{$t("retrogame.volume")}</span>
          <input
            type="range"
            min="0"
            max="100"
            bind:value={localSettings.audioVolume}
            class="slider"
          />
          <span class="value">{localSettings.audioVolume}%</span>
        </label>
      </div>
    {/if}

    {#if activeTab === "controls"}
      <div class="section">
        <div class="section-header">
          <h3>{$t("retrogame.keyMapping")}</h3>
          <Button variant="ghost" size="sm" onclick={resetKeyMapping}>
            <Icon icon="mdi:refresh" width={16} />
            {$t("retrogame.reset")}
          </Button>
        </div>
        <div class="key-grid">
          {#each Object.entries(localSettings.keyMapping) as [key, value]}
            <div class="key-item">
              <span class="key-label">{getKeyLabel(key as keyof KeyMapping)}</span>
              <input
                type="text"
                class="key-input"
                value={value}
                readonly
                onkeydown={(e) => {
                  e.preventDefault();
                  localSettings.keyMapping[key as keyof KeyMapping] = e.key;
                }}
              />
            </div>
          {/each}
        </div>
        <p class="hint">{$t("retrogame.keyBindHint")}</p>
      </div>

      <div class="section">
        <h3>{$t("retrogame.gamepad")}</h3>
        <p class="info">
          <Icon icon="mdi:information" width={16} />
          {$t("retrogame.gamepadAutoDetect")}
        </p>
      </div>
    {/if}

    {#if activeTab === "video"}
      <div class="section">
        <h3>{$t("retrogame.videoFilter")}</h3>
        <div class="filter-grid">
          {#each VIDEO_FILTERS as filter}
            <button
              class="filter-item"
              class:active={localSettings.videoFilter === filter.id}
              onclick={() => (localSettings.videoFilter = filter.id)}
            >
              <Icon
                icon={filter.id === "none"
                  ? "mdi:image"
                  : filter.id === "crt"
                    ? "mdi:television-classic"
                    : filter.id === "scanlines"
                      ? "mdi:view-sequential"
                      : filter.id === "smooth"
                        ? "mdi:blur"
                        : "mdi:grid"}
                width={24}
              />
              <span>{filter.name}</span>
            </button>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <!-- 操作按钮 -->
  <div class="actions">
    <Button variant="ghost" onclick={resetAll}>{$t("retrogame.restoreDefault")}</Button>
    <div class="actions-right">
      <Button variant="ghost" onclick={onCancel}>{$t("common.cancel")}</Button>
      <Button onclick={handleSave}>{$t("common.save")}</Button>
    </div>
  </div>
</div>

<style>
  .game-settings {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .tabs {
    display: flex;
    gap: 4px;
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 8px;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    border: none;
    background: none;
    color: var(--text-secondary);
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
  }

  .tab:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .tab.active {
    background: var(--color-primary-bg);
    color: var(--color-primary);
  }

  .content {
    display: flex;
    flex-direction: column;
    gap: 20px;
    max-height: 400px;
    overflow-y: auto;
    padding-right: 8px;
  }

  .section {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .section h3 {
    font-size: 13px;
    font-weight: 600;
    margin: 0;
    color: var(--text-secondary);
  }

  .hint {
    font-size: 12px;
    color: var(--text-tertiary);
    margin: 0;
  }

  .info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
    background: var(--bg-tertiary);
    padding: 8px 12px;
    border-radius: 6px;
    margin: 0;
  }

  .option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
  }

  .option span {
    font-size: 13px;
  }

  .slider {
    flex: 1;
    max-width: 150px;
    margin: 0 12px;
  }

  .value {
    width: 40px;
    text-align: right;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .key-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }

  .key-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .key-label {
    font-size: 11px;
    color: var(--text-tertiary);
  }

  .key-input {
    padding: 6px 10px;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    background: var(--bg-tertiary);
    color: var(--text-primary);
    font-size: 12px;
    text-align: center;
    cursor: pointer;
  }

  .key-input:focus {
    border-color: var(--color-primary);
    outline: none;
  }

  .filter-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

  .filter-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    padding: 12px;
    border: 1px solid var(--border-color);
    border-radius: 8px;
    background: var(--bg-secondary);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.2s;
  }

  .filter-item:hover {
    border-color: var(--color-primary);
    color: var(--text-primary);
  }

  .filter-item.active {
    border-color: var(--color-primary);
    background: var(--color-primary-bg);
    color: var(--color-primary);
  }

  .filter-item span {
    font-size: 12px;
  }

  .actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: 12px;
    border-top: 1px solid var(--border-color);
  }

  .actions-right {
    display: flex;
    gap: 8px;
  }

  .rom-dir-display {
    padding: 10px 12px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border-color);
    border-radius: 6px;
  }

  .rom-dir-display .dir-path {
    font-size: 13px;
    color: var(--text-secondary);
    word-break: break-all;
  }
</style>
