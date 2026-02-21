<script lang="ts">
  import Icon from "@iconify/svelte";
  import type { Album } from "$shared/services/photos";

  interface Props {
    albums: Album[];
    onOpen: (album: Album) => void;
  }

  let { albums, onOpen }: Props = $props();

  function formatDate(dateStr: string): string {
    const date = new Date(dateStr);
    return date.toLocaleDateString("zh-CN", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  }
</script>

<div class="album-grid">
  {#each albums as album (album.id)}
    <button
      class="album-card"
      onclick={() => onOpen(album)}
    >
      <div class="album-cover">
        {#if album.cover_url}
          <img src={album.cover_url} alt={album.name} />
        {:else}
          <div class="empty-cover">
            <Icon icon="mdi:folder-image" width={48} />
          </div>
        {/if}
      </div>
      <div class="album-info">
        <span class="album-name">{album.name}</span>
        <span class="album-meta">
          {album.photo_count} 张
          {#if album.description}
            · {album.description}
          {/if}
        </span>
      </div>
    </button>
  {/each}
</div>

<style>
  .album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 16px;
  }

  .album-card {
    background: var(--bg-secondary, #222);
    border: 1px solid var(--border-color, #333);
    border-radius: 12px;
    overflow: hidden;
    cursor: pointer;
    transition: transform 0.15s ease, box-shadow 0.15s ease;
    text-align: left;
    padding: 0;
  }

  .album-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
  }

  .album-cover {
    aspect-ratio: 4 / 3;
    background: var(--bg-tertiary, #2a2a2a);
    overflow: hidden;
  }

  .album-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .empty-cover {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary, #666);
  }

  .album-info {
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .album-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary, #fff);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .album-meta {
    font-size: 12px;
    color: var(--text-secondary, #888);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
