<script lang="ts">
  import Icon from "@iconify/svelte";
  import type { Board } from "./types";

  let { boards, selectedBoard, onSelect }: {
    boards: Board[];
    selectedBoard: string;
    onSelect: (path: string) => void;
  } = $props();

  let search = $state("");
  let filterArch = $state("");

  let architectures = $derived([...new Set(boards.map((b) => b.arch))].sort());

  let filtered = $derived.by(() => {
    let list = boards;
    if (filterArch) list = list.filter((b) => b.arch === filterArch);
    if (search.trim()) {
      const kw = search.trim().toLowerCase();
      list = list.filter(
        (b) =>
          b.name.toLowerCase().includes(kw) ||
          b.arch.toLowerCase().includes(kw) ||
          b.full_path.toLowerCase().includes(kw),
      );
    }
    return list;
  });
</script>

<div class="board-selector">
  <div class="toolbar">
    <div class="search-box">
      <Icon icon="mdi:magnify" width={16} />
      <input type="text" bind:value={search} placeholder="搜索开发板..." />
    </div>
    <div class="arch-filter">
      <button class="arch-btn" class:active={filterArch === ""} onclick={() => (filterArch = "")}>
        全部
      </button>
      {#each architectures as arch}
        <button class="arch-btn" class:active={filterArch === arch} onclick={() => (filterArch = arch)}>
          {arch}
        </button>
      {/each}
    </div>
    <span class="result-count">{filtered.length} / {boards.length}</span>
  </div>

  <div class="board-grid">
    {#each filtered as board (board.full_path)}
      <button
        class="board-card"
        class:selected={board.full_path === selectedBoard}
        onclick={() => onSelect(board.full_path)}
      >
        <div class="board-top">
          <Icon icon="mdi:developer-board" width={22} class="board-icon" />
          <span class="board-name">{board.name}</span>
          {#if board.full_path === selectedBoard}
            <span class="check"><Icon icon="mdi:check-circle" width={18} /></span>
          {/if}
        </div>
        <div class="board-tags">
          <span class="tag arch">{board.arch}</span>
          {#if board.cpu}<span class="tag">{board.cpu}</span>{/if}
          {#if board.linux}<span class="tag">v{board.linux}</span>{/if}
        </div>
      </button>
    {/each}
  </div>
</div>

<style>
  .board-selector {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    gap: 0;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color);
    background: var(--bg-window-header);
    flex-wrap: wrap;
  }

  .search-box {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 12px;
    border-radius: 8px;
    background: var(--bg-input);
    border: 1px solid var(--border-color);
    flex: 0 1 260px;
    color: var(--text-muted);
  }

  .search-box input {
    border: none;
    background: none;
    outline: none;
    font-size: 13px;
    color: var(--text-primary);
    width: 100%;
  }

  .search-box input::placeholder {
    color: var(--text-muted);
  }

  .arch-filter {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }

  .arch-btn {
    padding: 4px 12px;
    border-radius: 6px;
    font-size: 12px;
    border: 1px solid var(--border-color);
    background: var(--bg-secondary);
    color: var(--text-secondary);
    cursor: pointer;
    transition: all 0.15s;
  }

  .arch-btn:hover {
    border-color: var(--color-primary);
    color: var(--text-primary);
  }

  .arch-btn.active {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: #fff;
  }

  .result-count {
    margin-left: auto;
    font-size: 12px;
    color: var(--text-muted);
  }

  .board-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 10px;
    padding: 16px;
    overflow-y: auto;
    flex: 1;
  }

  .board-card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 14px;
    border-radius: 10px;
    border: 1px solid var(--border-color);
    background: var(--bg-card);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s;
  }

  .board-card:hover {
    border-color: var(--color-primary);
    background: var(--bg-hover);
  }

  .board-card.selected {
    border-color: var(--color-primary);
    background: var(--bg-active);
  }

  .board-top {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  :global(.board-icon) {
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .board-card.selected :global(.board-icon) {
    color: var(--color-primary);
  }

  .board-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .check {
    color: var(--color-primary);
    flex-shrink: 0;
  }

  .board-tags {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .tag {
    font-size: 11px;
    padding: 2px 8px;
    border-radius: 6px;
    background: var(--bg-secondary);
    color: var(--text-muted);
    font-family: "JetBrains Mono", "Fira Code", monospace;
  }

  .tag.arch {
    background: var(--color-primary-light);
    color: var(--color-primary);
    font-weight: 500;
  }
</style>
