<script lang="ts">
  import { t } from "svelte-i18n";
  import { workspaces, type Workspace } from "$desktop/stores";

  interface Props {
    visible?: boolean;
    onclose?: () => void;
  }

  let { visible = $bindable(false), onclose }: Props = $props();

  let container = $state<HTMLDivElement | undefined>(undefined);
  let hoveredWorkspace: string | null = $state(null);
  let editingId: string | null = $state(null);
  let editingName = $state("");

  // 工作区预览网格配置
  const previewWidth = 200;
  const previewHeight = 120;

  // 关闭面板
  function close() {
    visible = false;
    editingId = null;
    onclose?.();
  }

  // 处理键盘事件
  function handleKeydown(e: KeyboardEvent) {
    if (!visible) return;

    if (e.key === "Escape") {
      close();
    } else if (e.key === "ArrowLeft") {
      workspaces.prev();
    } else if (e.key === "ArrowRight") {
      workspaces.next();
    } else if (e.key === "Enter" && editingId === null) {
      close();
    }
  }

  // 切换到工作区
  function selectWorkspace(id: string) {
    workspaces.switchTo(id);
    close();
  }

  // 添加新工作区
  function addWorkspace() {
    const id = workspaces.create();
    if (id) {
      workspaces.switchTo(id);
    }
  }

  // 删除工作区
  function deleteWorkspace(e: MouseEvent, id: string) {
    e.stopPropagation();
    workspaces.delete(id);
  }

  // 开始编辑名称
  function startEditing(e: MouseEvent, workspace: Workspace) {
    e.stopPropagation();
    editingId = workspace.id;
    editingName = workspace.name;
  }

  // 保存编辑的名称
  function saveEditing() {
    if (editingId && editingName.trim()) {
      workspaces.rename(editingId, editingName.trim());
    }
    editingId = null;
  }

  // 处理输入框键盘事件
  function handleInputKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      saveEditing();
    } else if (e.key === "Escape") {
      editingId = null;
    }
    e.stopPropagation();
  }

  $effect(() => {
    if (visible) {
      document.addEventListener("keydown", handleKeydown);
      return () => document.removeEventListener("keydown", handleKeydown);
    }
  });
</script>

{#if visible}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="workspace-switcher" onclick={close}>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="switcher-container" bind:this={container} onclick={(e) => e.stopPropagation()}>
      <div class="switcher-header">
        <h2>{$t("desktop.workspace.title")}</h2>
        <p class="hint">{$t("desktop.workspace.hint")}</p>
      </div>

      <div class="workspaces-grid">
        {#each workspaces.all as workspace (workspace.id)}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="workspace-preview"
            class:active={workspace.id === workspaces.activeId}
            class:hovered={hoveredWorkspace === workspace.id}
            style:width="{previewWidth}px"
            style:height="{previewHeight}px"
            onclick={() => selectWorkspace(workspace.id)}
            onmouseenter={() => (hoveredWorkspace = workspace.id)}
            onmouseleave={() => (hoveredWorkspace = null)}
          >
            <div class="preview-content">
              <!-- 窗口缩略图预览 -->
              {#if workspace.windowIds.length === 0}
                <span class="empty-hint">{$t("desktop.workspace.empty")}</span>
              {:else}
                <span class="window-count">{$t("desktop.workspace.windowCount", { values: { n: workspace.windowIds.length } })}</span>
              {/if}
            </div>

            <div class="preview-footer">
              {#if editingId === workspace.id}
                <input
                  type="text"
                  class="name-input"
                  bind:value={editingName}
                  onblur={saveEditing}
                  onkeydown={handleInputKeydown}
                />
              {:else}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <span class="workspace-name" ondblclick={(e) => startEditing(e, workspace)}>
                  {workspace.name}
                </span>
              {/if}

              {#if workspaces.count > 1 && hoveredWorkspace === workspace.id}
                <button class="delete-btn" onclick={(e) => deleteWorkspace(e, workspace.id)}>
                  ×
                </button>
              {/if}
            </div>
          </div>
        {/each}

        <!-- 添加工作区按钮 -->
        {#if workspaces.count < workspaces.maxWorkspaces}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            class="workspace-preview add-workspace"
            style:width="{previewWidth}px"
            style:height="{previewHeight}px"
            onclick={addWorkspace}
          >
            <div class="add-icon">+</div>
            <span>{$t("desktop.workspace.new")}</span>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .workspace-switcher {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    backdrop-filter: blur(10px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10000;
    animation: fadeIn 0.2s ease-out;
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  .switcher-container {
    background: rgba(30, 30, 30, 0.95);
    border-radius: 16px;
    padding: 24px 32px;
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
    animation: slideIn 0.3s ease-out;
    max-width: 90vw;
    max-height: 80vh;
    overflow: auto;
  }

  @keyframes slideIn {
    from {
      transform: scale(0.95) translateY(-20px);
      opacity: 0;
    }
    to {
      transform: scale(1) translateY(0);
      opacity: 1;
    }
  }

  .switcher-header {
    text-align: center;
    margin-bottom: 24px;
  }

  .switcher-header h2 {
    margin: 0 0 8px 0;
    font-size: 24px;
    font-weight: 600;
    color: #fff;
  }

  .hint {
    margin: 0;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.5);
  }

  .workspaces-grid {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
    justify-content: center;
  }

  .workspace-preview {
    background: rgba(50, 50, 50, 0.8);
    border-radius: 12px;
    border: 2px solid transparent;
    cursor: pointer;
    transition: all 0.2s ease;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .workspace-preview:hover {
    border-color: rgba(255, 255, 255, 0.3);
    transform: scale(1.02);
  }

  .workspace-preview.active {
    border-color: #0078d4;
    box-shadow: 0 0 0 2px rgba(0, 120, 212, 0.3);
  }

  .preview-content {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
    font-size: 12px;
    color: rgba(255, 255, 255, 0.5);
  }

  .empty-hint {
    opacity: 0.5;
  }

  .window-count {
    background: rgba(0, 120, 212, 0.3);
    padding: 4px 12px;
    border-radius: 12px;
    font-size: 12px;
    color: #fff;
  }

  .preview-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: rgba(0, 0, 0, 0.3);
    min-height: 32px;
  }

  .workspace-name {
    color: #fff;
    font-size: 12px;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .name-input {
    flex: 1;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.3);
    border-radius: 4px;
    padding: 2px 8px;
    color: #fff;
    font-size: 12px;
    outline: none;
  }

  .name-input:focus {
    border-color: #0078d4;
  }

  .delete-btn {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    border: none;
    background: rgba(255, 0, 0, 0.5);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    line-height: 1;
    padding: 0;
    margin-left: 8px;
    transition: background 0.15s ease;
  }

  .delete-btn:hover {
    background: rgba(255, 0, 0, 0.8);
  }

  .add-workspace {
    border: 2px dashed rgba(255, 255, 255, 0.2);
    background: transparent;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: rgba(255, 255, 255, 0.5);
    font-size: 13px;
  }

  .add-workspace:hover {
    border-color: rgba(255, 255, 255, 0.4);
    color: rgba(255, 255, 255, 0.8);
    background: rgba(255, 255, 255, 0.05);
  }

  .add-icon {
    font-size: 36px;
    line-height: 1;
    font-weight: 300;
  }
</style>
