<script lang="ts">
  import Icon from "@iconify/svelte";
  import type { Photo } from "$shared/services/photos";
  import LazyThumbnail from "$shared/components/LazyThumbnail.svelte";

  interface Props {
    photos: Photo[];
    selectedIds: Set<string>;
    onSelect: (id: string) => void;
    onClick: (photo: Photo) => void;
    size?: number;
  }

  let { photos, selectedIds, onSelect, onClick, size = 160 }: Props = $props();

  function handleClick(e: MouseEvent, photo: Photo) {
    if (e.ctrlKey || e.metaKey || e.shiftKey) {
      e.preventDefault();
      onSelect(photo.id);
    } else {
      onClick(photo);
    }
  }

  function handleCheckboxClick(e: MouseEvent, id: string) {
    e.stopPropagation();
    onSelect(id);
  }

  function formatDuration(seconds: number): string {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, "0")}`;
  }
</script>

<div class="photo-grid" style="--item-size: {size}px;">
  {#each photos as photo (photo.id)}
    {@const isSelected = selectedIds.has(photo.id)}
    <div
      class="photo-item"
      class:selected={isSelected}
      onclick={(e) => handleClick(e, photo)}
      role="button"
      tabindex="0"
    >
      <!-- 选择框 -->
      <div
        class="checkbox"
        class:visible={selectedIds.size > 0 || isSelected}
        onclick={(e) => handleCheckboxClick(e, photo.id)}
      >
        {#if isSelected}
          <Icon icon="mdi:checkbox-marked" width={22} />
        {:else}
          <Icon icon="mdi:checkbox-blank-outline" width={22} />
        {/if}
      </div>

      <!-- 缩略图 -->
      <LazyThumbnail
        src={photo.thumbnail_url}
        alt={photo.filename}
        isVideo={photo.type === "video"}
        size={size}
        fallbackIcon={photo.type === "video" ? "mdi:video" : "mdi:image"}
      />

      <!-- 视频时长 -->
      {#if photo.type === "video" && photo.duration > 0}
        <div class="duration">
          <Icon icon="mdi:play" width={12} />
          {formatDuration(photo.duration)}
        </div>
      {/if}

      <!-- 收藏标记 -->
      {#if photo.is_favorite}
        <div class="favorite-badge">
          <Icon icon="mdi:heart" width={16} />
        </div>
      {/if}

      <!-- 悬浮信息 -->
      <div class="hover-info">
        <span class="filename">{photo.filename}</span>
      </div>
    </div>
  {/each}
</div>

<style>
  .photo-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(var(--item-size), 1fr));
    gap: 8px;
  }

  .photo-item {
    position: relative;
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    background: var(--bg-tertiary, #2a2a2a);
    transition: transform 0.15s ease, box-shadow 0.15s ease;
  }

  .photo-item:hover {
    transform: scale(1.02);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }

  .photo-item.selected {
    outline: 3px solid var(--accent-color, #0066cc);
    outline-offset: -3px;
  }

  .photo-item :global(.lazy-thumbnail) {
    width: 100%;
    height: 100%;
  }

  .photo-item :global(.lazy-thumbnail img) {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  /* 选择框 */
  .checkbox {
    position: absolute;
    top: 8px;
    left: 8px;
    z-index: 10;
    color: #fff;
    opacity: 0;
    transition: opacity 0.15s ease;
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.5));
  }

  .photo-item:hover .checkbox,
  .checkbox.visible {
    opacity: 1;
  }

  .checkbox:hover {
    transform: scale(1.1);
  }

  /* 视频时长 */
  .duration {
    position: absolute;
    bottom: 8px;
    right: 8px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px;
    background: rgba(0, 0, 0, 0.7);
    border-radius: 4px;
    font-size: 11px;
    color: #fff;
  }

  /* 收藏标记 */
  .favorite-badge {
    position: absolute;
    top: 8px;
    right: 8px;
    color: #ff4757;
    filter: drop-shadow(0 1px 2px rgba(0, 0, 0, 0.5));
  }

  /* 悬浮信息 */
  .hover-info {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 24px 8px 8px;
    background: linear-gradient(transparent, rgba(0, 0, 0, 0.7));
    opacity: 0;
    transition: opacity 0.15s ease;
  }

  .photo-item:hover .hover-info {
    opacity: 1;
  }

  .filename {
    font-size: 11px;
    color: #fff;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
  }
</style>
