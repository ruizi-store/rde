<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    text?: string;
    position?: "top" | "bottom" | "left" | "right";
    delay?: number;
    children: Snippet;
  }

  let { text = "", position = "top", delay = 200, children }: Props = $props();

  let visible = $state(false);
  let timeoutId: ReturnType<typeof setTimeout> | null = null;

  function show() {
    timeoutId = setTimeout(() => {
      visible = true;
    }, delay);
  }

  function hide() {
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    visible = false;
  }
</script>

<div
  class="tooltip-wrapper"
  onmouseenter={show}
  onmouseleave={hide}
  onfocus={show}
  onblur={hide}
  role="presentation"
>
  {@render children()}
  {#if text && visible}
    <div class="tooltip tooltip-{position}">
      {text}
      <span class="tooltip-arrow"></span>
    </div>
  {/if}
</div>

<style>
  .tooltip-wrapper {
    position: relative;
    display: inline-flex;
  }

  .tooltip {
    position: absolute;
    z-index: 10001;
    padding: 6px 10px;
    background: rgba(0, 0, 0, 0.85);
    color: white;
    font-size: 12px;
    border-radius: 4px;
    white-space: nowrap;
    pointer-events: none;
    animation: fadeIn 0.15s ease-out;
    max-width: 250px;
    word-wrap: break-word;
    line-height: 1.4;
  }

  .tooltip-arrow {
    position: absolute;
    width: 0;
    height: 0;
    border: 5px solid transparent;
  }

  .tooltip-top {
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 8px;

    .tooltip-arrow {
      top: 100%;
      left: 50%;
      transform: translateX(-50%);
      border-top-color: rgba(0, 0, 0, 0.85);
    }
  }

  .tooltip-bottom {
    top: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-top: 8px;

    .tooltip-arrow {
      bottom: 100%;
      left: 50%;
      transform: translateX(-50%);
      border-bottom-color: rgba(0, 0, 0, 0.85);
    }
  }

  .tooltip-left {
    right: 100%;
    top: 50%;
    transform: translateY(-50%);
    margin-right: 8px;

    .tooltip-arrow {
      left: 100%;
      top: 50%;
      transform: translateY(-50%);
      border-left-color: rgba(0, 0, 0, 0.85);
    }
  }

  .tooltip-right {
    left: 100%;
    top: 50%;
    transform: translateY(-50%);
    margin-left: 8px;

    .tooltip-arrow {
      right: 100%;
      top: 50%;
      transform: translateY(-50%);
      border-right-color: rgba(0, 0, 0, 0.85);
    }
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
</style>
