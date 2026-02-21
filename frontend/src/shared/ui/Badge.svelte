<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    count?: number;
    max?: number;
    dot?: boolean;
    showZero?: boolean;
    variant?: "default" | "secondary" | "success" | "warning" | "error";
    offset?: [number, number];
    children?: Snippet;
  }

  let {
    count = 0,
    max = 99,
    dot = false,
    showZero = false,
    variant = "default",
    offset = [0, 0],
    children,
  }: Props = $props();

  let displayCount = $derived(count > max ? `${max}+` : count.toString());
  let visible = $derived(dot || count > 0 || (count === 0 && showZero));
</script>

{#if children}
  <div class="badge-wrapper">
    {@render children()}
    {#if visible}
      <span
        class="badge badge-{variant}"
        class:dot
        style="transform: translate({offset[0]}px, {offset[1]}px)"
      >
        {#if !dot}{displayCount}{/if}
      </span>
    {/if}
  </div>
{:else if visible}
  <span class="badge badge-{variant} standalone" class:dot>
    {#if !dot}{displayCount}{/if}
  </span>
{/if}

<style>
  .badge-wrapper {
    position: relative;
    display: inline-flex;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    font-weight: 500;
    line-height: 1;
    white-space: nowrap;
    border-radius: 10px;
    padding: 2px 6px;
    min-width: 20px;
    height: 20px;
    box-sizing: border-box;

    &:not(.standalone) {
      position: absolute;
      top: 0;
      right: 0;
      transform: translate(50%, -50%);
      box-shadow: 0 0 0 2px white;
    }

    &.dot {
      width: 8px;
      height: 8px;
      min-width: 8px;
      padding: 0;
      border-radius: 50%;
    }

    &.standalone {
      vertical-align: middle;
    }
  }

  .badge-default {
    background: var(--color-primary, #4a90d9);
    color: white;
  }

  .badge-secondary {
    background: #9e9e9e;
    color: white;
  }

  .badge-success {
    background: #52c41a;
    color: white;
  }

  .badge-warning {
    background: #faad14;
    color: white;
  }

  .badge-error {
    background: #ff4d4f;
    color: white;
  }
</style>
