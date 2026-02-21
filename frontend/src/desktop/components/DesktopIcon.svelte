<script lang="ts">
  import { t } from "svelte-i18n";
  import type { DesktopIcon } from "$desktop/stores/desktop.svelte";
  import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
  import { desktop } from "$desktop/stores/desktop.svelte";
  import { toast } from "$shared/stores/toast.svelte";
  import AppContextMenu from "./AppContextMenu.svelte";

  let { icon }: { icon: DesktopIcon } = $props();

  let isSelected = $derived(desktop.selectedIconId === icon.id);
  let launching = $state(false);

  // 右键菜单状态
  let contextMenu = $state<{ x: number; y: number } | null>(null);

  // 拖拽状态
  let dragging = $state(false);
  let dragStartPos = $state({ cx: 0, cy: 0, ox: 0, oy: 0 }); // cx/cy: cursor, ox/oy: icon origin
  let dragOffset = $state({ dx: 0, dy: 0 });
  let dragTarget = $state<{ x: number; y: number } | null>(null);
  const DRAG_THRESHOLD = 5;
  let dragReady = $state(false); // pointer down but not yet dragging

  // 网格常量
  const CELL_W = 88; // 80 + 8 gap
  const CELL_H = 98; // 90 + 8 gap
  const PAD = 16;

  // 获取对应的应用信息
  let app = $derived(apps.get(icon.appId));

  // 获取应用的本地化名称
  function getDisplayName(): string {
    if (app) {
      const key = `apps.names.${app.id}`;
      const translated = $t(key);
      if (translated !== key) return translated;
    }
    return icon.name;
  }

  // 为右键菜单创建应用定义（包括动态应用）
  let appForContextMenu = $derived.by<ExtendedAppDefinition | null>(() => {
    // 已注册的应用直接使用
    if (app) {
      return app;
    }

    return null;
  });

  function handleClick(e: MouseEvent) {
    if (dragging) return;
    e.stopPropagation();
    desktop.selectIcon(icon.id);
  }

  async function handleDoubleClick(e: MouseEvent) {
    if (dragging) return;
    e.preventDefault();
    e.stopPropagation();

    if (launching) return;

    launching = true;

    try {
      await apps.launch(icon.appId);
    } catch (err: any) {
      toast.error(err.message || $t("desktop.launchFailed"));
    } finally {
      launching = false;
      desktop.selectIcon(null);
    }
  }

  function handleContextMenu(e: MouseEvent) {
    if (dragging) return;
    e.preventDefault();
    e.stopPropagation();
    desktop.selectIcon(icon.id);
    contextMenu = { x: e.clientX, y: e.clientY };
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  // ========== 拖拽逻辑 ==========
  function handlePointerDown(e: PointerEvent) {
    if (e.button !== 0) return; // 仅左键
    e.stopPropagation();
    dragReady = true;
    dragStartPos = {
      cx: e.clientX,
      cy: e.clientY,
      ox: icon.x,
      oy: icon.y,
    };
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }

  function handlePointerMove(e: PointerEvent) {
    if (!dragReady && !dragging) return;

    const dx = e.clientX - dragStartPos.cx;
    const dy = e.clientY - dragStartPos.cy;

    if (!dragging) {
      if (Math.abs(dx) > DRAG_THRESHOLD || Math.abs(dy) > DRAG_THRESHOLD) {
        dragging = true;
        desktop.selectIcon(icon.id);
      } else {
        return;
      }
    }

    dragOffset = { dx, dy };

    // 计算目标网格坐标
    const targetX = Math.max(0, Math.round(dragStartPos.ox + dx / CELL_W));
    const targetY = Math.max(0, Math.round(dragStartPos.oy + dy / CELL_H));
    dragTarget = { x: targetX, y: targetY };
  }

  function handlePointerUp(e: PointerEvent) {
    if (dragging && dragTarget) {
      desktop.moveIcon(icon.id, dragTarget.x, dragTarget.y);
    }
    dragging = false;
    dragReady = false;
    dragTarget = null;
    dragOffset = { dx: 0, dy: 0 };
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="desktop-icon"
  class:selected={isSelected}
  class:dragging
  onclick={handleClick}
  ondblclick={handleDoubleClick}
  oncontextmenu={handleContextMenu}
  onpointerdown={handlePointerDown}
  onpointermove={handlePointerMove}
  onpointerup={handlePointerUp}
  style="grid-column: {icon.x + 1}; grid-row: {icon.y + 1};{dragging ? ` transform: translate(${dragOffset.dx}px, ${dragOffset.dy}px); z-index: 1000;` : ''}"
>
  <div class="icon-image">
    <img
      src={icon.icon}
      alt={getDisplayName()}
      draggable="false"
      onerror={(e) => ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
    />
  </div>
  <span class="icon-name">{getDisplayName()}</span>
</div>

<!-- 拖拽时显示目标位置预览 -->
{#if dragging && dragTarget}
  <div
    class="drop-preview"
    style="grid-column: {dragTarget.x + 1}; grid-row: {dragTarget.y + 1};"
  ></div>
{/if}

<!-- 右键菜单 -->
{#if contextMenu && appForContextMenu}
  <AppContextMenu
    x={contextMenu.x}
    y={contextMenu.y}
    app={appForContextMenu}
    context="desktop"
    onclose={closeContextMenu}
  />
{/if}

<style>
  .desktop-icon {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-start;
    width: 80px;
    min-height: 90px;
    padding: 6px 4px;
    border-radius: 6px;
    cursor: pointer;
    user-select: none;
    transition: opacity 0.2s;
    touch-action: none; /* 允许 pointer events 拖拽 */

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    &.selected {
      background: rgba(74, 144, 217, 0.3);
      outline: 1px solid rgba(74, 144, 217, 0.6);
    }

    &.dragging {
      opacity: 0.8;
      cursor: grabbing;
      transition: none;
      pointer-events: auto;
    }

    &.disabled {
      opacity: 0.5;

      .icon-image img {
        filter: grayscale(100%) drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
      }
    }
  }

  .icon-image {
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 4px;
    position: relative;

    img {
      max-width: 100%;
      max-height: 100%;
      filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
      transition: filter 0.2s;
    }

    .disabled-overlay {
      position: absolute;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;

      &::after {
        content: "";
        width: 20px;
        height: 20px;
        background: #ff9800;
        border-radius: 50%;
        position: absolute;
        bottom: -4px;
        right: -4px;
        box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
      }
    }
  }

  .icon-name {
    font-size: 12px;
    color: white;
    text-align: center;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
    line-height: 1.3;
    max-width: 100%;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    word-break: break-word;
  }

  .drop-preview {
    width: 80px;
    height: 90px;
    border-radius: 6px;
    border: 2px dashed rgba(74, 144, 217, 0.6);
    background: rgba(74, 144, 217, 0.1);
    pointer-events: none;
  }
</style>
