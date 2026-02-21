<script lang="ts">
  let {
    visible = false,
    zone = null as
      | "left"
      | "right"
      | "top-left"
      | "top-right"
      | "bottom-left"
      | "bottom-right"
      | "maximize"
      | null,
  }: {
    visible?: boolean;
    zone?:
      | "left"
      | "right"
      | "top-left"
      | "top-right"
      | "bottom-left"
      | "bottom-right"
      | "maximize"
      | null;
  } = $props();

  // 计算预览区域样式
  let style = $derived(() => {
    if (!zone) return "";

    const taskbarHeight = 48;
    const screenHeight = `calc(100vh - ${taskbarHeight}px)`;

    switch (zone) {
      case "left":
        return `top: 0; left: 0; width: 50%; height: ${screenHeight};`;
      case "right":
        return `top: 0; right: 0; width: 50%; height: ${screenHeight};`;
      case "top-left":
        return `top: 0; left: 0; width: 50%; height: calc(${screenHeight} / 2);`;
      case "top-right":
        return `top: 0; right: 0; width: 50%; height: calc(${screenHeight} / 2);`;
      case "bottom-left":
        return `bottom: ${taskbarHeight}px; left: 0; width: 50%; height: calc(${screenHeight} / 2);`;
      case "bottom-right":
        return `bottom: ${taskbarHeight}px; right: 0; width: 50%; height: calc(${screenHeight} / 2);`;
      case "maximize":
        return `top: 0; left: 0; width: 100%; height: ${screenHeight};`;
      default:
        return "";
    }
  });
</script>

{#if visible && zone}
  <div class="snap-preview" style={style()}>
    <div class="snap-preview-inner"></div>
  </div>
{/if}

<style>
  .snap-preview {
    position: fixed;
    z-index: 9999;
    pointer-events: none;
    padding: 8px;
    animation: fadeIn 0.15s ease;

    @keyframes fadeIn {
      from {
        opacity: 0;
        transform: scale(0.95);
      }
      to {
        opacity: 1;
        transform: scale(1);
      }
    }
  }

  .snap-preview-inner {
    width: 100%;
    height: 100%;
    background: rgba(74, 144, 217, 0.2);
    border: 2px solid rgba(74, 144, 217, 0.5);
    border-radius: 8px;
    backdrop-filter: blur(4px);
  }
</style>
